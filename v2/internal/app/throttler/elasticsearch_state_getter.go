package throttler

import (
	"context"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client.
	"github.com/pkg/errors"                 // Wrap errors with stacktraces.
	"golang.org/x/sync/errgroup"            // Cancel multiple goroutines if one fails.

	"github.com/mintel/elasticsearch-asg/v2/pkg/es" // Extensions to the Elasticsearch client.
)

// ElasticsearchState represents the state of an Elasticsearch
// cluster in the context of the throttler app.
type ElasticsearchState struct {
	// One of: "red", "yellow", "green".
	Status string

	// True if shards are being moved from one node to another.
	RelocatingShards bool

	// True if indices are recovering from data
	// stored on disk, such as during a node reboot.
	RecoveringFromStore bool
}

// ElasticsearchStateGetter queries an Elasticsearch cluster to return
// status information about the cluster that is useful when deciding
// whether to allow scaling up or down of the Elasticsearch cluster.
type ElasticsearchStateGetter struct {
	client *elastic.Client
}

// NewElasticsearchStateGetter returns a new ElasticsearchStateGetter.
func NewElasticsearchStateGetter(client *elastic.Client) *ElasticsearchStateGetter {
	return &ElasticsearchStateGetter{
		client: client,
	}
}

// Get returns an ElasticsearchState.
func (sg *ElasticsearchStateGetter) Get() (*ElasticsearchState, error) {
	cs := &ElasticsearchState{}
	g, ctx := errgroup.WithContext(context.Background())

	g.Go(func() error {
		hs := sg.client.ClusterHealth()

		resp, err := hs.Do(ctx)
		if err != nil {
			return errors.Wrap(err, "error GET /_cluster/health")
		}
		cs.Status = resp.Status
		cs.RelocatingShards = resp.RelocatingShards > 0
		return nil
	})

	g.Go(func() error {
		rs := es.NewIndicesRecoveryService(sg.client).
			ActiveOnly(true).
			Detailed(false)
		resp, err := rs.Do(ctx)
		if err != nil {
			return errors.Wrap(err, "error GET /_cat/recovery")
		}
		for _, idx := range resp {
			for _, s := range idx.Shards {
				if s.Type == "store" {
					cs.RecoveringFromStore = true
					return nil
				}
			}
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return cs, nil
}
