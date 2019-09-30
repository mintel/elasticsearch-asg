package healthchecker

import (
	"context"

	"github.com/mintel/healthcheck"         // Healthchecks framework.
	elastic "github.com/olivere/elastic/v7" // Elasticsearch client.
	"github.com/pkg/errors"                 // Wrap errors with stacktrace.
)

// CheckReadyJoinedCluster checks if a Elasticsearch node has joined a cluster.
func CheckReadyJoinedCluster(c *elastic.Client) healthcheck.Check {
	return func() error {
		resp, err := c.ClusterState().Do(context.Background())
		if err != nil {
			return errors.Wrap(err, "error getting cluster state")
		}
		if resp.StateUUID == "_na_" || resp.Version == -1 {
			return errors.New("node has not joined cluster")
		}
		return nil
	}
}
