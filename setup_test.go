package esasg

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

const testhost = "http://127.0.0.1:9200"

// setupLogging sets up zap test logging and returns a teardown function.
func setupLogging(t *testing.T) func() {
	logger := zaptest.NewLogger(t)
	f1 := zap.ReplaceGlobals(logger)
	f2 := zap.RedirectStdLog(logger)
	teardown := func() {
		f2()
		f1()
		if err := logger.Sync(); err != nil {
			panic(err)
		}
	}
	return teardown
}
