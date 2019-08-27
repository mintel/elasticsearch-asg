package healthchecker

import (
	"context"
	"sort"

	"github.com/mintel/healthcheck" // Healthchecks framework
	elastic "github.com/olivere/elastic/v7"
	"github.com/pkg/errors"
	"go.uber.org/zap" // Logging

	"github.com/mintel/elasticsearch-asg/pkg/es" // Extensions to the Elasticsearch client
)

// CheckReadyRollingUpgrade checks that Elasticsearch has recovered from a rolling upgrade.
//
// The purpose of this check is to prevent the rolling upgrade from proceeding.
// Most deployment systems have some concept of a healthcheck grace period
// where a failing health check is ignored for some period of time during startup.
// The rolling upgrade usually won't proceed until the healthcheck starts passing.
//
// The check fails for one of two reasons:
// 1. Index shards on this node are in the INITIALIZING state.
//    Only shards that are present when the check first runs
//    are considered in future runs. That way any newly created indices/shards
//    in the INITIALIZING state won't interrupt node startup. (Really, you
//    should try not to write to Elasticsearch when doing an upgrade.)
// 2. Any shard in the cluster is in a RELOCATING state. The rolling
//    upgrade should not proceed while shards are being moved around
//    due to the danger of data loss.
//
// After the check passes for the first time, it will always pass on every subsequent call.
//
// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/rolling-upgrades.html
func CheckReadyRollingUpgrade(c *elastic.Client) healthcheck.Check {
	var nodeName string
	var initialShards []string
	doneOnce := false // disable after first success
	return func() error {
		if doneOnce {
			return nil
		}

		if nodeName == "" {
			info, err := c.NodesInfo().NodeId("_local").Metric("info").Do(context.Background())
			if err != nil {
				return errors.Wrap(err, "error getting node info")
			}
			if n := len(info.Nodes); n != 1 {
				zap.L().Panic("got incorrect number of nodes when requesting _local node info", zap.Int("num_nodes", n))
			}
			for _, node := range info.Nodes {
				nodeName = node.Name
				break
			}
		}

		shards, err := es.NewCatShardsService(c).Do(context.Background())
		if err != nil {
			return errors.Wrap(err, "error getting cluster shards")
		}

		if initialShards == nil {
			initialShards = make([]string, 0, len(shards))
			for _, shard := range shards {
				initialShards = append(initialShards, *shard.ID)
			}
			sort.Strings(initialShards)
		}

		for _, shard := range shards {
			if shard.State == "INITIALIZING" {
				if i := sort.SearchStrings(initialShards, *shard.ID); i < len(initialShards) && initialShards[i] == *shard.ID {
					return errors.New("shard is INITIALIZING")
				}
			}
			if shard.State == "RELOCATING" {
				return errors.New("shard is RELOCATING")
			}
		}

		doneOnce = true
		return nil
	}
}
