package esasg

import (
	"context"
	"errors"
	"strings"

	elastic "github.com/olivere/elastic/v7"
	"go.uber.org/zap"
	tomb "gopkg.in/tomb.v2"

	"github.com/mintel/elasticsearch-asg/pkg/es"
	"github.com/mintel/elasticsearch-asg/pkg/str"
)

// ErrInconsistentNodes is returned when ElasticsearchQueryService.Nodes()
// gets different sets of nodes from Elasticsearch across API calls.
var ErrInconsistentNodes = errors.New("got inconsistent nodes from Elasticsearch")

// In case of ErrInconsistentNodes, retry this many times before giving up.
const defaultInconsistentNodesRetries = 3

// ElasticsearchQueryService describes methods to get information from Elasticsearch.
type ElasticsearchQueryService interface {
	// Nodes returns info and stats about the nodes in the Elasticsearch cluster,
	// as a map from node name to Node.
	// If names are past, limit to nodes with those names.
	// It's left up to the caller to check if all the names are in the response.
	Nodes(ctx context.Context, names ...string) (map[string]*Node, error)

	// Node returns a single node with the given name.
	// Will return nil if the node doesn't exist.
	Node(ctx context.Context, name string) (*Node, error)
}

// elasticsearchQueryService implements the ElasticsearchQueryService interface
type elasticsearchQueryService struct {
	ElasticsearchQueryService

	client *elastic.Client
	logger *zap.Logger
}

// NewElasticsearchQueryService returns a new ElasticsearchQueryService.
func NewElasticsearchQueryService(client *elastic.Client) ElasticsearchQueryService {
	return &elasticsearchQueryService{
		client: client,
		logger: zap.L().Named("elasticsearchQueryService"),
	}
}

// Node returns a single node with the given name.
// Will return nil if the node doesn't exist.
func (s *elasticsearchQueryService) Node(ctx context.Context, name string) (*Node, error) {
	nodes, err := s.Nodes(ctx, name)
	if err != nil {
		return nil, err
	}
	return nodes[name], nil
}

// Nodes returns info and stats about the nodes in the Elasticsearch cluster,
// as a map from node name to Node.
// If names are past, limit to nodes with those names.
// It's left up to the caller to check if all the names are in the response.
func (s *elasticsearchQueryService) Nodes(ctx context.Context, names ...string) (map[string]*Node, error) {
	var result map[string]*Node
	var err error
	tries := defaultInconsistentNodesRetries
	for tryCounter := 0; tryCounter < tries; tryCounter++ {
		if tryCounter > 0 {
			zap.L().Warn("got error describing Elasticsearch nodes",
				zap.Error(err),
				zap.Int("try", tryCounter+1),
				zap.Int("max_tries", tries),
			)
		}
		result, err = s.nodes(ctx, names...)
		if err == nil {
			return result, nil
		}
	}
	return result, err
}

func (s *elasticsearchQueryService) nodes(ctx context.Context, names ...string) (map[string]*Node, error) {
	t, ctx := tomb.WithContext(ctx)

	var statsResp *elastic.NodesStatsResponse
	t.Go(func() error {
		var err error
		rs := s.client.NodesStats()
		if len(names) > 0 {
			rs = rs.NodeId(names...)
		}
		statsResp, err = rs.Do(ctx)
		return err
	})

	var infoResp *elastic.NodesInfoResponse
	t.Go(func() error {
		var err error
		rs := s.client.NodesInfo()
		if len(names) > 0 {
			rs = rs.NodeId(names...)
		}
		infoResp, err = rs.Do(ctx)
		return err
	})

	var shardsResp es.CatShardsResponse
	t.Go(func() error {
		var err error
		shardsResp, err = es.NewCatShardsService(s.client).Do(ctx)
		return err
	})

	var settings *shardAllocationExcludeSettings
	t.Go(func() error {
		resp, err := es.NewClusterGetSettingsService(s.client).FilterPath("*." + shardAllocExcludeSetting + ".*").Do(ctx)
		if err != nil {
			return err
		}
		settings = newShardAllocationExcludeSettings(resp.Persistent)
		tSettings := newShardAllocationExcludeSettings(resp.Transient)
		if len(tSettings.Name) > 0 {
			settings.Name = tSettings.Name
		}
		if len(tSettings.Host) > 0 {
			settings.Host = tSettings.Host
		}
		if len(tSettings.IP) > 0 {
			settings.IP = tSettings.IP
		}
		for k, v := range tSettings.Attr {
			if len(v) > 0 {
				settings.Attr[k] = v
			}
		}
		return nil
	})

	if err := t.Wait(); err != nil {
		return nil, err
	} else if len(statsResp.Nodes) != len(infoResp.Nodes) {
		statsNodes := make([]string, 0, len(statsResp.Nodes))
		for name := range statsResp.Nodes {
			statsNodes = append(statsNodes, name)
		}
		infoNodes := make([]string, 0, len(infoResp.Nodes))
		for name := range infoResp.Nodes {
			infoNodes = append(infoNodes, name)
		}
		zap.L().Error("got info and stats responses of different lengths",
			zap.Strings("stats_nodes", statsNodes),
			zap.Strings("info_nodes", infoNodes),
		)
		return nil, ErrInconsistentNodes
	}

	nodes := make(map[string]*Node, len(statsResp.Nodes))
	for _, ni := range infoResp.Nodes {
		ip := strings.Split(ni.IP, ":")[0] // Remove port number
		excluded := str.In(ni.Name, settings.Name...) || str.In(ip, settings.IP...) || str.In(ni.Host, settings.Host...)
		if !excluded {
			for a, v := range ni.Attributes {
				if sv, ok := settings.Attr[a]; ok && str.In(v, sv...) {
					excluded = true
					break
				}
			}
		}
		nodes[ni.Name] = &Node{
			ClusterName:             infoResp.ClusterName,
			NodesInfoNode:           *ni,
			ExcludedShardAllocation: excluded,
		}
	}
	for _, ns := range statsResp.Nodes {
		n, ok := nodes[ns.Name]
		if !ok {
			nodeNames := make([]string, 0, len(nodes))
			for name := range nodes {
				nodeNames = append(nodeNames, name)
			}
			zap.L().Error("got node in stats response that isn't in info response",
				zap.String("name", ns.Name),
				zap.Strings("nodes", nodeNames),
			)
			return nil, ErrInconsistentNodes
		}
		n.Stats = *ns
	}
	for _, sr := range shardsResp {
		shardNodes, err := parseShardNodes(sr.Node)
		if err != nil {
			zap.L().Error(err.Error(), zap.String("name", sr.Node))
			return nil, err
		} else if len(shardNodes) == 0 {
			// Unassigned shard. Ignore.
			continue
		}
		for _, node := range shardNodes {
			if n, ok := nodes[node]; ok {
				n.Shards = append(n.Shards, sr)
			} else if len(names) == 0 {
				nodeNames := make([]string, 0, len(nodes))
				for name := range nodes {
					nodeNames = append(nodeNames, name)
				}
				zap.L().Error("got node in shards response that isn't in info or stats response",
					zap.String("name", node),
					zap.Strings("nodes", nodeNames),
				)
				return nil, ErrInconsistentNodes
			}
		}
	}

	return nodes, nil
}

// parseShardNodes parses the node string response from the /_cat/shards endpoint.
// This could be one of:
// - An empty string for an unassigned shard.
// - A node name for an normal shard.
// - Multiple node names if the shard is being relocated.
func parseShardNodes(node string) ([]string, error) {
	if node == "" {
		return nil, nil
	}
	parts := strings.Fields(node)
	switch len(parts) {
	case 1:
		return parts, nil
	case 5: // Example: "i-0968d7621b79cd73d -> 10.2.4.58 kNe49LLvSqGXBn2s8Ffgyw i-0a2ed08df0e5cfff6"
		return []string{parts[0], parts[4]}, nil
	}
	return nil, errors.New("couldn't parse /_cat/shards response node name")
}
