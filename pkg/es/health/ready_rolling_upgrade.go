package health

import (
	"context"
	"errors"

	elastic "github.com/olivere/elastic/v7"
	"go.uber.org/zap"
)

func init() {
	RegisterReadyCheck("rolling-upgrade", CheckReadyRollingUpgrade)
}

// disableCheckReadyRollingUpgrade disables CheckReadyRollingUpgrade by making it a no-op.
var disableCheckReadyRollingUpgrade = false

// CheckReadyRollingUpgrade checks Elasticsearch cluster and index health, but only once during
// node start up. It succeeds once the cluster state is green, or the cluster state is yellow
// but no shards are being initialized or relocated. After the check passes for the first time,
// it disables itself by becoming a no-op.
func CheckReadyRollingUpgrade(ctx context.Context, client *elastic.Client, logger *zap.Logger) error {
	if disableCheckReadyRollingUpgrade {
		logger.Debug("disabled due to disableCheckReadyRollingUpgrade = true")
		return nil
	}

	resp, err := client.CatHealth().Do(ctx)
	if err != nil {
		logger.Info("error getting cluster health: ", zap.Error(err))
		return err
	}

	for _, row := range resp {
		rowLogger := logger.With(
			zap.String("cluster", row.Cluster),
			zap.String("status", row.Status),
			zap.Int("init", row.Init),
			zap.Int("relo", row.Relo),
		)
		switch {
		case row.Status == "green":
			rowLogger.Debug("cluster status is green")
			continue
		case row.Status == "red":
			const msg = "cluster status is red"
			rowLogger.Info(msg)
			return errors.New(msg)
		case row.Init > 0:
			const msg = "shards are initializing"
			rowLogger.Info(msg)
			return errors.New(msg)
		case row.Relo > 0:
			const msg = "shards are relocating"
			rowLogger.Info(msg)
			return errors.New(msg)
		}
	}

	disableCheckReadyRollingUpgrade = true
	return nil
}
