package health

import (
	"context"
	"errors"

	elastic "github.com/olivere/elastic/v7"
	"go.uber.org/zap"
)

func init() {
	RegisterLiveCheck("head", CheckLiveHEAD)
}

// CheckLiveHEAD checks if a HEAD request to / returns 200.
func CheckLiveHEAD(ctx context.Context, client *elastic.Client, logger *zap.Logger) error {
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
		logger.Info(msg, zap.Int("status_code", resp.StatusCode))
		return errors.New(msg)
	}

	logger.Info("HEAD request returned 200 OK", zap.Int("status_code", resp.StatusCode))
	return nil
}
