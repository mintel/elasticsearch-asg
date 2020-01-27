package health

import (
	"testing"
	"time"

	"go.uber.org/zap"         // Logging
	"go.uber.org/zap/zaptest" // Logging during tests
)

func setup(t *testing.T) (string, func()) {
	logger := zaptest.NewLogger(t)
	defer func() {
		_ = logger.Sync()
	}()
	t1 := zap.ReplaceGlobals(logger)
	t2 := zap.RedirectStdLog(logger)

	originalTimeout := DefaultHTTPTimeout
	DefaultHTTPTimeout = 500 * time.Millisecond

	return "http://127.0.0.1:9200", func() {
		DefaultHTTPTimeout = originalTimeout
		t2()
		t1()
		_ = logger.Sync()
	}
}
