package health

import (
	"context"
	"errors"

	"github.com/heptiolabs/healthcheck"

	"go.uber.org/zap"
)

// CheckReadyJoinedCluster checks if a Elasticsearch node has joined a cluster.
func CheckReadyJoinedCluster(ctx context.Context, url string) healthcheck.Check {
	lc := lazyClient{
		URL: url,
	}
	return func() error {
		logger := zap.L().Named("CheckReadyJoinedCluster")
		client, err := lc.Client()
		if err != nil {
			return err
		}
		resp, err := client.ClusterState().Do(ctx)
		if err != nil {
			zap.L().Info("joined-cluster: error getting cluster state", zap.Error(err))
			return err
		}

		logger = logger.With(zap.String("cluster_uuid", resp.StateUUID), zap.Int64("version", resp.Version))

		if resp.StateUUID == "_na_" || resp.Version == -1 {
			const msg = "joined-cluster: node has not joined cluster"
			logger.Info(msg)
			return errors.New(msg)
		}

		logger.Debug("joined-cluster: node has joined cluster")
		return nil
	}
}
