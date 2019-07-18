package health

import (
	"context"
	"testing"
	"time"

	"github.com/heptiolabs/healthcheck"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

const localhost = "http://127.0.0.1:9200"

func setup(t *testing.T, checkFactory func(context.Context, string) healthcheck.Check) (healthcheck.Check, func()) {
	logger := zaptest.NewLogger(t)
	defer func() {
		if err := logger.Sync(); err != nil {
			panic(err)
		}
	}()
	t1 := zap.ReplaceGlobals(logger)
	t2 := zap.RedirectStdLog(logger)

	ctx, cancel := context.WithCancel(context.Background())
	check := checkFactory(ctx, localhost)

	originalTimeout := DefaultHTTPTimeout
	DefaultHTTPTimeout = 500 * time.Millisecond

	return check, func() {
		DefaultHTTPTimeout = originalTimeout
		cancel()
		t2()
		t1()
		if err := logger.Sync(); err != nil {
			panic(err)
		}
	}
}
