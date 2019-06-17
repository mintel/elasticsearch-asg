package health

import (
	"context"
	"errors"

	"github.com/heptiolabs/healthcheck"

	"go.uber.org/zap"
)

// CheckReadyRollingUpgrade checks Elasticsearch cluster and index health, but only once during
// node start up. It succeeds once the cluster state is green, or the cluster state is yellow
// but no shards are being initialized or relocated. After the check passes for the first time,
// it disables itself by becoming a no-op.
//
// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/rolling-upgrades.html
func CheckReadyRollingUpgrade(ctx context.Context, url string) healthcheck.Check {
	doneOnce := false // disable after first success
	lc := lazyClient{
		URL: url,
	}
	return func() error {
		logger := zap.L().Named("CheckReadyRollingUpgrade")

		if doneOnce {
			logger.Debug("disabled due to doneOnce = true")
			return nil
		}

		client, err := lc.Client()
		if err != nil {
			return err
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

		doneOnce = true
		return nil
	}
}
