package esasg

import (
	"context"
	"errors"
	"strings"
	"sync"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap" // Logging

	"github.com/mintel/elasticsearch-asg/pkg/es"      // Elasticsearch client extensions
	"github.com/mintel/elasticsearch-asg/pkg/metrics" // Prometheus metrics
	"github.com/mintel/elasticsearch-asg/pkg/str"     // String utilities
)

// ErrInconsistentNodes is returned when ElasticsearchQueryService.Nodes()
// gets different sets of nodes from Elasticsearch across API calls.
var ErrInconsistentNodes = errors.New("got inconsistent nodes from Elasticsearch")

const (
	// In case of ErrInconsistentNodes, retry this many times before giving up.
	defaultInconsistentNodesRetries = 3

	querySubsystem = "query"
)

var (
	// elasticsearchQueryClusterNameDuration is the Prometheus metric for ElasticsearchQuery.ClusterName() durations.
	elasticsearchQueryClusterNameDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: querySubsystem,
		Name:      "cluster_name_request_seconds",
		Help:      "Requests to get Elasticsearch cluster name.",
		Buckets:   prometheus.DefBuckets,
	})

	// elasticsearchQueryClusterNameErrors is the Prometheus metric for ElasticsearchQuery.ClusterName() errors.
	elasticsearchQueryClusterNameErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: querySubsystem,
		Name:      "cluster_name_errors_total",
		Help:      "Requests to get Elasticsearch cluster name.",
	}, []string{metrics.LabelStatusCode})

	// elasticsearchQueryNodesDuration is the Prometheus metric for ElasticsearchQuery.Nodes() and Node() durations.
	elasticsearchQueryNodesDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: querySubsystem,
		Name:      "nodes_request_seconds",
		Help:      "Requests to get Elasticsearch nodes info.",
		Buckets:   prometheus.DefBuckets,
	})

	// elasticsearchQueryNodesErrors is the Prometheus metric for ElasticsearchQuery.Nodes() and Node() errors.
	elasticsearchQueryNodesErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: querySubsystem,
		Name:      "nodes_errors_total",
		Help:      "Requests to get Elasticsearch nodes info.",
	}, []string{metrics.LabelStatusCode})
)

// ElasticsearchQueryService implements methods that read from Elasticsearch endpoints.
type ElasticsearchQueryService struct {
	client *elastic.Client
	logger *zap.Logger
}

// NewElasticsearchQueryService returns a new ElasticsearchQueryService.
func NewElasticsearchQueryService(client *elastic.Client) *ElasticsearchQueryService {
	return &ElasticsearchQueryService{
		client: client,
		logger: zap.L().Named("ElasticsearchQueryService"),
	}
}

// ClusterName return the name of the Elasticsearch cluster.
func (s *ElasticsearchQueryService) ClusterName(ctx context.Context) (string, error) {
	timer := prometheus.NewTimer(elasticsearchQueryClusterNameDuration)
	defer timer.ObserveDuration()
	resp, err := s.client.ClusterHealth().Do(ctx)
	if err != nil {
		elasticsearchQueryClusterNameErrors.WithLabelValues(metrics.ElasticsearchStatusCode(err)).Inc()
		return "", err
	}
	return resp.ClusterName, nil
}

// Node returns a single node with the given name.
// Will return nil if the node doesn't exist.
func (s *ElasticsearchQueryService) Node(ctx context.Context, name string) (*Node, error) {
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
func (s *ElasticsearchQueryService) Nodes(ctx context.Context, names ...string) (map[string]*Node, error) {
	timer := prometheus.NewTimer(elasticsearchQueryNodesDuration)
	defer timer.ObserveDuration()

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
			elasticsearchQueryNodesErrors.WithLabelValues(metrics.ElasticsearchStatusCode(err)).Inc()
			return result, nil
		}
	}
	return result, err
}

func (s *ElasticsearchQueryService) nodes(ctx context.Context, names ...string) (map[string]*Node, error) {
	// We collect information from 4 Elasticsearch endpoints.
	// The requests are send concurrently by separate goroutines.
	var statsResp *elastic.NodesStatsResponse    // Node stats
	var infoResp *elastic.NodesInfoResponse      // Node info
	var shardsResp es.CatShardsResponse          // Shards
	var settings *shardAllocationExcludeSettings // Cluster settings

	wg := sync.WaitGroup{}      // Counter of running goroutines that can be waited on.
	wg.Add(4)                   // Add 4 because 4 goroutines.
	done := make(chan struct{}) // Channel that gets closed to signal that all goroutines are done.
	errc := make(chan error, 4) // Channel that is used by the goroutines to send any errors that occur.
	go func() {
		wg.Wait()   // Once all goroutines finish...
		close(done) // close done to signal the parent goroutine.
	}()

	ctx, cancel := context.WithCancel(ctx) // If there's an error in one of the goroutines, abort the rest by sharing a common context.
	defer cancel()                         // Early return due to error will trigger this.

	// Get node stats
	go func() {
		defer wg.Done() // Decrement goroutine counter on goroutine end.
		var err error
		statsResp, err = s.client.NodesStats().NodeId(names...).Do(ctx)
		if err != nil {
			errc <- err
		}
	}()

	// Get node info
	go func() {
		defer wg.Done() // Decrement goroutine counter on goroutine end.
		var err error
		infoResp, err = s.client.NodesInfo().NodeId(names...).Do(ctx)
		if err != nil {
			errc <- err
		}
	}()

	// Get shards
	go func() {
		defer wg.Done() // Decrement goroutine counter on goroutine end.
		var err error
		shardsResp, err = es.NewCatShardsService(s.client).Do(ctx)
		if err != nil {
			errc <- err
		}
	}()

	// Get cluster settings
	go func() {
		defer wg.Done() // Decrement goroutine counter on goroutine end.
		resp, err := es.NewClusterGetSettingsService(s.client).FilterPath("*." + shardAllocExcludeSetting + ".*").Do(ctx)
		if err != nil {
			errc <- err
			return
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
	}()

	// Wait for an error, or all the goroutines to finish.
	select {
	case err := <-errc:
		return nil, err
	case <-done:
		close(errc) // Release resources.
	}

	// Check if results have the same number of nodes.
	if len(statsResp.Nodes) != len(infoResp.Nodes) {
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

	// Merge all endpoint responses into a single set of Nodes.
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

// parseShardNodes parses the node name from the /_cat/shards endpoint response
//
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
