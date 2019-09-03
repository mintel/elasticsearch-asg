package elasticsearch

import (
	"context"
	"errors"
	"strings"
	"sync"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap" // Logging

	"github.com/mintel/elasticsearch-asg/internal/pkg/metrics" // Prometheus metrics
	"github.com/mintel/elasticsearch-asg/pkg/ctxlog"           // Logger from context
	"github.com/mintel/elasticsearch-asg/pkg/es"               // Elasticsearch client extensions
	"github.com/mintel/elasticsearch-asg/pkg/str"              // String utilities
)

// ErrInconsistentNodes is returned when Query.Nodes()
// gets different sets of nodes from Elasticsearch across API calls.
var ErrInconsistentNodes = errors.New("got inconsistent nodes from Elasticsearch")

const querySubsystem = "query"

var (
	// QueryClusterNameDuration is the Prometheus metric for Query.ClusterName() durations.
	QueryClusterNameDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: querySubsystem,
		Name:      "cluster_name_request_duration_seconds",
		Help:      "Requests to get Elasticsearch cluster name.",
		Buckets:   prometheus.DefBuckets,
	}, []string{metrics.LabelStatus})

	// QueryNodesDuration is the Prometheus metric for Query.Nodes() and Node() durations.
	QueryNodesDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: querySubsystem,
		Name:      "nodes_request_duration_seconds",
		Help:      "Requests to get Elasticsearch nodes info.",
		Buckets:   prometheus.DefBuckets,
	}, []string{metrics.LabelStatus})

	// QueryGetSnapshotsDuration is the Prometheus metric for Query.GetSnapshots() durations.
	QueryGetSnapshotsDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: querySubsystem,
		Name:      "get_snapshots_request_duration_seconds",
		Help:      "Requests to list Elasticsearch snapshots.",
		Buckets:   prometheus.DefBuckets,
	}, []string{metrics.LabelStatus})
)

// Query implements methods that read from Elasticsearch endpoints.
type Query struct {
	client *elastic.Client
}

// NewQuery returns a new Query.
func NewQuery(client *elastic.Client) *Query {
	return &Query{
		client: client,
	}
}

// ClusterName return the name of the Elasticsearch cluster.
func (q *Query) ClusterName(ctx context.Context) (name string, err error) {
	timer := metrics.NewVecTimer(QueryClusterNameDuration)
	defer timer.ObserveErr(err)

	logger := ctxlog.L(ctx).Named("Query.ClusterName")

	logger.Debug("getting cluster name")
	var resp *elastic.ClusterHealthResponse
	resp, err = q.client.ClusterHealth().Do(ctx)
	if err != nil {
		logger.Error("error getting cluster name", zap.Error(err))
		return
	}
	name = resp.ClusterName
	logger.Debug("got cluster name", zap.String("cluster", name))
	return
}

// Node returns a single node with the given name.
// Will return nil if the node doesn't exist.
func (q *Query) Node(ctx context.Context, name string) (node *Node, err error) {
	timer := metrics.NewVecTimer(QueryNodesDuration)
	defer timer.ObserveErr(err)
	ctx = ctxlog.WithName(ctx, "Query.Node")
	var nodes map[string]*Node
	nodes, err = q.nodes(ctx, name)
	if err != nil {
		return
	}
	node = nodes[name]
	return
}

// Nodes returns info and stats about the nodes in the Elasticsearch cluster,
// as a map from node name to Node.
// If names are passed, limit to nodes with those names.
// It's left up to the caller to check if all the names are in the response.
func (q *Query) Nodes(ctx context.Context, names ...string) (nodes map[string]*Node, err error) {
	timer := metrics.NewVecTimer(QueryNodesDuration)
	defer timer.ObserveErr(err)
	ctx = ctxlog.WithName(ctx, "Query.Nodes")
	nodes, err = q.nodes(ctx, names...)
	return
}

func (q *Query) nodes(ctx context.Context, names ...string) (map[string]*Node, error) {
	logger := ctxlog.L(ctx).Named("Query.ClusterName")

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
		logger.Debug("getting nodes stats")
		statsResp, err = q.client.NodesStats().NodeId(names...).Do(ctx)
		if err != nil {
			logger.Error("error getting nodes stats", zap.Error(err))
			errc <- err
		} else {
			logger.Debug("got nodes stats", zap.Int("count", len(statsResp.Nodes)))
		}
	}()

	// Get node info
	go func() {
		defer wg.Done() // Decrement goroutine counter on goroutine end.
		var err error
		logger.Debug("getting nodes info")
		infoResp, err = q.client.NodesInfo().NodeId(names...).Do(ctx)
		if err != nil {
			logger.Error("error getting nodes info", zap.Error(err))
			errc <- err
		} else {
			logger.Debug("got nodes info", zap.Int("count", len(infoResp.Nodes)))
		}
	}()

	// Get shards
	go func() {
		defer wg.Done() // Decrement goroutine counter on goroutine end.
		var err error
		logger.Debug("getting shards info")
		shardsResp, err = es.NewCatShardsService(q.client).Do(ctx)
		if err != nil {
			logger.Error("error getting shards info", zap.Error(err))
			errc <- err
		} else {
			logger.Debug("got shards info", zap.Int("count", len(shardsResp)))
		}
	}()

	// Get cluster settings
	go func() {
		defer wg.Done() // Decrement goroutine counter on goroutine end.
		logger.Debug("getting cluster settings")
		resp, err := es.NewClusterGetSettingsService(q.client).FilterPath("*." + shardAllocExcludeSetting + ".*").Do(ctx)
		if err != nil {
			logger.Error("error getting cluster settings", zap.Error(err))
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
		logger.Debug("got cluster settings")
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
		logger.Error("got info and stats responses of different lengths",
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
			logger.Error("got node in stats response that isn't in info response",
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
			logger.Error(err.Error(), zap.String("name", sr.Node))
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
				logger.Error("got node in shards response that isn't in info or stats response",
					zap.String("name", node),
					zap.Strings("nodes", nodeNames),
				)
				return nil, ErrInconsistentNodes
			}
		}
	}

	return nodes, nil
}

// GetSnapshots returns a list of all snapshots in a given Elasticsearch snapshot repository.
// If names are passed response is limited to snapshots with those names.
func (q *Query) GetSnapshots(ctx context.Context, repoName string, names ...string) (snapshots []*elastic.Snapshot, err error) {
	timer := metrics.NewVecTimer(QueryGetSnapshotsDuration)
	defer timer.ObserveErr(err)

	logger := ctxlog.L(ctx).Named("Query.GetSnapshots").With(zap.String("repository_name", repoName))

	var resp *elastic.SnapshotGetResponse
	s := q.client.SnapshotGet(repoName)
	if len(names) != 0 {
		s.Snapshot(names...)
	}

	logger.Debug("getting snapshots")
	if resp, err = s.Do(ctx); err != nil {
		logger.Error("error getting snapshots")
	} else {
		snapshots = resp.Snapshots
		logger.Debug("got snapshots", zap.Int("count", len(snapshots)))
	}
	return
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
