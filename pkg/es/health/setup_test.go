package health

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"         // Logging
	"go.uber.org/zap/zaptest" // Logging during tests

	"github.com/mintel/elasticsearch-asg/pkg/ctxlog"
)

func setup(t *testing.T) (context.Context, string, func()) {
	logger := zaptest.NewLogger(t)
	defer func() {
		_ = logger.Sync()
	}()
	t1 := zap.ReplaceGlobals(logger)
	t2 := zap.RedirectStdLog(logger)

	ctx, cancel := context.WithCancel(context.Background())
	ctx = ctxlog.WithLogger(ctx, logger)

	originalTimeout := DefaultHTTPTimeout
	DefaultHTTPTimeout = 500 * time.Millisecond

	return ctx, "http://127.0.0.1:9200", func() {
		DefaultHTTPTimeout = originalTimeout
		cancel()
		t2()
		t1()
		_ = logger.Sync()
	}
}
