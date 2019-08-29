package health

import (
	"context"
	"errors"

	"github.com/heptiolabs/healthcheck"               // Healthchecks framework
	"github.com/mintel/elasticsearch-asg/pkg/metrics" // Prometheus metrics
	elastic "github.com/olivere/elastic/v7"           // Elasticsearch client
	"go.uber.org/zap"                                 // Logging

	"github.com/mintel/elasticsearch-asg/pkg/ctxlog" // Logger from context
)

// CheckLiveHEAD checks if a HEAD request to / returns 200.
func CheckLiveHEAD(ctx context.Context, URL string) healthcheck.Check {
	lc := lazyClient{
		URL: URL,
	}
	return func() error {
		logger := ctxlog.L(ctx).Named("CheckLiveHEAD")
		client, err := lc.Client()
		if err != nil {
			return err
		}
		resp, err := client.PerformRequest(ctx, elastic.PerformRequestOptions{
			Method: "HEAD",
			Path:   "/",
		})
		if err != nil {
			logger.Info("Error communicating with Elasticsearch", zap.Error(err))
			return err
		}

		if resp.StatusCode != 200 {
			const msg = "HEAD request returned non-200 status code"
			logger.Debug(msg, zap.Int(metrics.LabelStatusCode, resp.StatusCode))
			return errors.New(msg)
		}

		logger.Info("HEAD request returned 200 OK", zap.Int(metrics.LabelStatusCode, resp.StatusCode))
		return nil
	}
}
