package esasg

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	elastic "github.com/olivere/elastic/v7"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
	tomb "gopkg.in/tomb.v2"

	"github.com/mintel/elasticsearch-asg/pkg/es"
	"github.com/mintel/elasticsearch-asg/pkg/str"
)

const shardAllocExcludeSetting = "cluster.routing.allocation.exclude"

var (
	// ErrInconsistentNodes is returned when ElasticsearchService.Nodes()
	// gets different sets of nodes from Elasticsearch across API calls.
	ErrInconsistentNodes = errors.New("got inconsistent nodes from Elasticsearch")

	// In case of ErrInconsistentNodes, retry this many times before giving up.
	defaultInconsistentNodesRetries = 3
)

// ElasticsearchService provides methods to perform some common tasks
// that the lower-level elastic client doesn't provide.
type ElasticsearchService struct {
	client *elastic.Client
	logger *zap.Logger
}

// NewElasticsearchService creates a new ElasticsearchService.
func NewElasticsearchService(client *elastic.Client) *ElasticsearchService {
	return &ElasticsearchService{
		logger: zap.L().Named("ElasticsearchService"),
		client: client,
	}
}

// Nodes returns info and stats about the nodes in the Elasticsearch cluster,
// as a map from node name to Node.
// If names are past, limit to nodes with those names.
func (s *ElasticsearchService) Nodes(ctx context.Context, names ...string) (map[string]*Node, error) {
	var result map[string]*Node
	var err error
	tries := defaultInconsistentNodesRetries
	for tryCounter := 0; tryCounter < tries; tryCounter++ {
		result, err = s.nodes(ctx, names...)
		if err == nil {
			return result, nil
		}
		zap.L().Warn("got error describing Elasticsearch nodes",
			zap.Error(err),
			zap.Int("try", tryCounter+1),
			zap.Int("max_tries", tries),
		)
	}
	return result, err
}

func (s *ElasticsearchService) nodes(ctx context.Context, names ...string) (map[string]*Node, error) {
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
		settings = newshardAllocationExcludeSettings(resp.Persistent)
		tSettings := newshardAllocationExcludeSettings(resp.Transient)
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
	}

	if len(statsResp.Nodes) != len(infoResp.Nodes) {
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
			return nil, ErrInconsistentNodes
		}
		n.Stats = *ns
	}
	for _, sr := range shardsResp {
		n, ok := nodes[sr.Node]
		if !ok {
			return nil, ErrInconsistentNodes
		}
		n.Shards = append(n.Shards, sr)
	}

	return nodes, nil
}

// Node returns a single node with the given name.
func (s *ElasticsearchService) Node(ctx context.Context, name string) (*Node, error) {
	nodes, err := s.Nodes(ctx, name)
	if err != nil {
		return nil, err
	} else if len(nodes) != 1 {
		return nil, fmt.Errorf("got wrong number of nodes (%d)", len(nodes))
	}
	n, ok := nodes[name]
	if !ok {
		for k := range nodes {
			return nil, fmt.Errorf("got node with wrong name (%s)", k)
		}
	}
	return n, nil
}

// Drain excludes a node from shard allocation, which will cause Elasticsearch
// to remove shards from the node until empty.
func (s *ElasticsearchService) Drain(ctx context.Context, nodeName string) error {
	resp, err := es.NewClusterGetSettingsService(s.client).Do(ctx)
	if err != nil {
		return err
	}
	settings := newshardAllocationExcludeSettings(resp.Transient)
	settings.Name = append(settings.Name, nodeName)
	sort.Strings(settings.Name)
	// ignore everything but name
	settings.IP = nil
	settings.Host = nil
	settings.Attr = nil
	_, err = es.NewClusterPutSettingsService(s.client).BodyJSON(map[string]interface{}{"transient": settings.Map()}).Do(ctx)
	return err
}

// Undrain reverses Drain.
func (s *ElasticsearchService) Undrain(ctx context.Context, nodeName string) error {
	resp, err := es.NewClusterGetSettingsService(s.client).Do(ctx)
	if err != nil {
		return err
	}
	settings := newshardAllocationExcludeSettings(resp.Transient)
	found := false
	filtered := settings.Name[:0]
	for _, name := range settings.Name {
		if name == nodeName {
			found = true
		} else {
			filtered = append(filtered, name)
		}
	}
	if !found {
		return nil
	}
	sort.Strings(filtered)
	settings.Name = filtered
	// ignore everything but name
	settings.IP = nil
	settings.Host = nil
	settings.Attr = nil
	_, err = es.NewClusterPutSettingsService(s.client).BodyJSON(map[string]interface{}{"transient": settings.Map()}).Do(ctx)
	return err
}

// Node represents info and stats about an Elasticsearch node at a point in time.
type Node struct {
	elastic.NodesInfoNode

	ClusterName             string
	ElectedMaster           bool
	ExcludedShardAllocation bool
	Stats                   elastic.NodesStatsNode
	Shards                  es.CatShardsResponse
}

// NewNodeFromName creates a new Node with the given name.
func NewNodeFromName(name string) *Node {
	return &Node{
		NodesInfoNode: elastic.NodesInfoNode{
			Name: name,
		},
	}
}

// Indices returns list of of index names present on this shard.
func (n *Node) Indices() []string {
	m := make(map[string]struct{})
	for _, s := range n.Shards {
		m[s.Index] = struct{}{}
	}
	indices := make([]string, 0, len(m))
	for i := range m {
		indices = append(indices, i)
	}
	sort.Strings(indices)
	return indices
}

// shardAllocationExcludeSettings represents the transient shard allocation exclusions
// of an Elasticsearch cluster.
type shardAllocationExcludeSettings struct {
	Name, Host, IP []string
	Attr           map[string][]string
}

// newshardAllocationExcludeSettings creates a new shardAllocationExcludeSettings.
func newshardAllocationExcludeSettings(settings *gjson.Result) *shardAllocationExcludeSettings {
	s := &shardAllocationExcludeSettings{
		Attr: make(map[string][]string),
	}
	settings.Get(shardAllocExcludeSetting).ForEach(func(key, value gjson.Result) bool {
		k := key.String()
		v := strings.Split(value.String(), ",")
		switch k {
		case "_name":
			s.Name = v
		case "_ip":
			s.IP = v
		case "_host":
			s.Host = v
		default:
			s.Attr[k] = v
		}
		return true
	})
	return s
}

func (s *shardAllocationExcludeSettings) Map() map[string]interface{} {
	m := make(map[string]interface{})
	if s.Name != nil {
		if len(s.Name) == 0 {
			m[shardAllocExcludeSetting+"._name"] = nil
		} else {
			m[shardAllocExcludeSetting+"._name"] = strings.Join(s.Name, ",")
		}
	}
	if s.Host != nil {
		if len(s.Host) == 0 {
			m[shardAllocExcludeSetting+"._host"] = nil
		} else {
			m[shardAllocExcludeSetting+"._host"] = strings.Join(s.Host, ",")
		}
	}
	if s.IP != nil {
		if len(s.IP) == 0 {
			m[shardAllocExcludeSetting+"._ip"] = nil
		} else {
			m[shardAllocExcludeSetting+"._ip"] = strings.Join(s.IP, ",")
		}
	}
	for k, v := range s.Attr {
		if len(v) == 0 {
			m[shardAllocExcludeSetting+"."+k] = nil
		} else {
			m[shardAllocExcludeSetting+"."+k] = strings.Join(v, ",")
		}
	}
	return m
}
