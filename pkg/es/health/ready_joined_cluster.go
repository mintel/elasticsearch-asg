package health

import (
	"context"
	"errors"

	"github.com/heptiolabs/healthcheck" // Healthchecks framework
	"go.uber.org/zap"                   // Logging
)

// CheckReadyJoinedCluster checks if a Elasticsearch node has joined a cluster.
func CheckReadyJoinedCluster(url string) healthcheck.Check {
	lc := lazyClient{
		URL: url,
	}
	return func() error {
		logger := zap.L().Named("CheckReadyJoinedCluster")
		client, err := lc.Client()
		if err != nil {
			return err
		}
		resp, err := client.ClusterState().Do(context.Background())
		if err != nil {
			zap.L().Info("error getting cluster state", zap.Error(err))
			return err
		}

		logger = logger.With(zap.String("cluster_uuid", resp.StateUUID), zap.Int64("version", resp.Version))

		if resp.StateUUID == "_na_" || resp.Version == -1 {
			const msg = "node has not joined cluster"
			logger.Info(msg)
			return errors.New(msg)
		}

		logger.Debug("node has joined cluster")
		return nil
	}
}
