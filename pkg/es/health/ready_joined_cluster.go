package health

import (
	"context"
	"errors"

	elastic "github.com/olivere/elastic/v7"
	"go.uber.org/zap"
)

func init() {
	RegisterReadyCheck("joined-cluster", CheckReadyJoinedCluster)
}

// CheckReadyJoinedCluster checks if a Elasticsearch node has joined a cluster.
func CheckReadyJoinedCluster(ctx context.Context, client *elastic.Client, logger *zap.Logger) error {
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
