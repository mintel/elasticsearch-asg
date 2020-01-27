package drainer

import (
	"context"
	"sort"
	"sync"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client.
	"github.com/pkg/errors"                 // Wrap errors with stacktrace.
	"golang.org/x/sync/errgroup"            // Cancel multiple goroutines if one fails.

	"github.com/mintel/elasticsearch-asg/v2/pkg/es" // Extensions to the Elasticsearch client.
)

// ElasticsearchFacadeIface is an interface for Elasticsearch
// so it can be mocked during tests.
type ElasticsearchFacadeIface interface {
	GetState(context.Context) (*ClusterState, error)
	DrainNodes(context.Context, []string) error
	UndrainNodes(context.Context, []string) error
}

// ElasticsearchFacade provides a facade for the drainer app's
// interactions with the ElasticsearchFacade API.
type ElasticsearchFacade struct {
	c          *elastic.Client
	settingsMu sync.RWMutex // Protect access to the Elasticsearch cluster settings API.
}

var _ ElasticsearchFacadeIface = (*ElasticsearchFacade)(nil) // Assert ElasticsearchFacade implements the ElasticsearchFacadeIface interface.

// NewElasticsearchFacade returns a new ElasticsearchFacade.
func NewElasticsearchFacade(c *elastic.Client) *ElasticsearchFacade {
	return &ElasticsearchFacade{
		c: c,
	}
}

// GetState returns a ClusterState representing the current
// state of the Elasticsearch cluster.
func (e *ElasticsearchFacade) GetState(ctx context.Context) (*ClusterState, error) {
	e.settingsMu.RLock()
	defer e.settingsMu.RUnlock()

	var (
		info     *elastic.NodesInfoResponse
		shards   es.CatShardsResponse
		settings *es.ClusterGetSettingsResponse
	)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		info, err = e.c.NodesInfo().Metric("http").Do(ctx)
		return errors.Wrap(err, "error getting nodes info")
	})

	g.Go(func() error {
		var err error
		shards, err = es.NewCatShardsService(e.c).Do(ctx)
		return errors.Wrap(err, "error getting shards")
	})

	g.Go(func() error {
		var err error
		settings, err = es.NewClusterGetSettingsService(e.c).
			FilterPath("*." + es.ShardAllocExcludeSetting + ".*").
			Do(ctx)
		return errors.Wrap(err, "error getting cluster settings")
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	state := NewClusterState(info, shards, settings)
	return state, nil
}

// DrainNodes puts Elasticsearch nodes into a draining state by adding their
// names to the list of nodes excluded for data shard allocation. Once set,
// Elasticsearch will being moving any data shards to other nodes.
//
// See also: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/allocation-filtering.html
func (e *ElasticsearchFacade) DrainNodes(ctx context.Context, names []string) error {
	if len(names) == 0 {
		return nil
	}

	e.settingsMu.Lock()
	defer e.settingsMu.Unlock()

	settings, err := es.NewClusterGetSettingsService(e.c).
		FilterPath("*." + es.ShardAllocExcludeSetting + ".*").
		Do(context.Background())
	if err != nil {
		return errors.Wrap(err, "error getting cluster settings")
	}

	exclusions := es.NewShardAllocationExcludeSettings(settings.Transient)
	sort.Strings(exclusions.Name)
	for _, name := range names {
		i := sort.SearchStrings(exclusions.Name, name) // Index in sorted slice where name should be.
		if i < len(exclusions.Name) && exclusions.Name[i] == name {
			// Node is already excluded from allocation.
			return nil
		}
		// Insert name into slice (https://github.com/golang/go/wiki/SliceTricks#insert)
		exclusions.Name = append(exclusions.Name, "")
		copy(exclusions.Name[i+1:], exclusions.Name[i:])
		exclusions.Name[i] = name
	}

	// Ignore all node exclusion attributes other than node name.
	exclusions.IP = nil
	exclusions.Host = nil
	exclusions.Attr = nil

	// Update cluster settings with new shard allocation exclusions.
	settingsMap := exclusions.Map()
	body := map[string]map[string]*string{"transient": settingsMap}
	_, err = es.NewClusterPutSettingsService(e.c).BodyJSON(body).Do(context.Background())
	if err != nil {
		return errors.Wrap(err, "error putting cluster settings")
	}

	return nil
}

// UndrainNodes reverses DrainNodes by removing from the list of nodes
// excluded from shard allocation.
//
// See also: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/allocation-filtering.html
func (e *ElasticsearchFacade) UndrainNodes(ctx context.Context, names []string) error {
	if len(names) == 0 {
		return nil
	}

	e.settingsMu.Lock()
	defer e.settingsMu.Unlock()

	settings, err := es.NewClusterGetSettingsService(e.c).
		FilterPath("*." + es.ShardAllocExcludeSetting + ".*").
		Do(context.Background())
	if err != nil {
		return errors.Wrap(err, "error getting cluster settings")
	}

	exclusions := es.NewShardAllocationExcludeSettings(settings.Transient)
	sort.Strings(exclusions.Name)
	for _, name := range names {
		i := sort.SearchStrings(exclusions.Name, name) // Index in sorted slice where name should be.
		if i == len(exclusions.Name) || exclusions.Name[i] != name {
			// Node is already absent from the shard allocation exclusion list.
			return nil
		}
		// Remove nodeName from slice (https://github.com/golang/go/wiki/SliceTricks#delete)
		exclusions.Name = exclusions.Name[:i+copy(exclusions.Name[i:], exclusions.Name[i+1:])]
	}

	// Ignore all node exclusion attributes other than node name.
	exclusions.IP = nil
	exclusions.Host = nil
	exclusions.Attr = nil

	// Update cluster settings with new shard allocation exclusions.
	settingsMap := exclusions.Map()
	body := map[string]map[string]*string{"transient": settingsMap}
	_, err = es.NewClusterPutSettingsService(e.c).BodyJSON(body).Do(context.Background())
	if err != nil {
		return errors.Wrap(err, "error putting cluster settings")
	}

	return nil
}
