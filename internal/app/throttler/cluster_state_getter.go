package throttler

import (
	"context"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client.
	"golang.org/x/sync/errgroup"            // Cancel multiple goroutines if one fails.

	"github.com/mintel/elasticsearch-asg/pkg/es" // Extensions to the Elasticsearch client.
)

// ClusterStateGetter queries an Elasticsearch cluster to return
// status information about the cluster that is useful when deciding
// whether to allow scaling up or down of the Elasticsearch cluster.
type ClusterStateGetter struct {
	client *elastic.Client
}

// NewClusterStateGetter returns a new ClusterStateGetter.
func NewClusterStateGetter(client *elastic.Client) *ClusterStateGetter {
	return &ClusterStateGetter{
		client: client,
	}
}

// Get returns a ClusterState. If the previous call to Get returned
// a ClusterState whose Status was "red" or had relocating shards, this
// call will block until the cluster status is "yellow" or "green" and
// there are no relocating shards.
//
// Only one call to Get can proceed at a time. Concurrent calls will block.
func (sg *ClusterStateGetter) Get() (*ClusterState, error) {
	cs := &ClusterState{}
	g, ctx := errgroup.WithContext(context.Background())

	g.Go(func() error {
		hs := sg.client.ClusterHealth()

		resp, err := hs.Do(ctx)
		if err != nil {
			return err
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
			return err
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
