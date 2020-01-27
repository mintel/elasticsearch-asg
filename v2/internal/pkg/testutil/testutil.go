// Package testutil contains miscellaneous testing utilities.
package testutil

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"path/filepath"
	"testing"

	"go.uber.org/zap" // Logging.
	"go.uber.org/zap/zaptest"
	gock "gopkg.in/h2non/gock.v1" // HTTP request mocking.
)

// TestLogger returns a zap Logger that logs all messages to the given testing.TB.
// It replaces the zap global Logger and redirects the stdlib log to the test Logger.
func TestLogger(t *testing.T) (logger *zap.Logger, teardown func()) {
	logger = zaptest.NewLogger(t)
	teardownLogger1 := zap.ReplaceGlobals(logger)
	teardownLogger2 := zap.RedirectStdLog(logger)
	teardown = func() {
		teardownLogger2()
		teardownLogger1()
		_ = logger.Sync()
	}
	return
}

// GockLogObserver returns a gock.ObserverFunc that logs HTTP requests to a zap Logger.
func GockLogObserver(logger *zap.Logger) gock.ObserverFunc {
	return func(request *http.Request, mock gock.Mock) {
		bytes, _ := httputil.DumpRequestOut(request, true)
		logger.Debug("gock intercepted http request",
			zap.String("request", string(bytes)),
			zap.Bool("matches_mock", mock != nil),
		)
	}
}

// LoadTestData is a helper to load test data from a `testdata` directory relaive to the CWD.
func LoadTestData(name string) string {
	path := filepath.Join("testdata", name) // relative path
	data, err := ioutil.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("failed to load test data file %s: %s", name, err))
	}
	return string(data)
}

// ClientTestSetup sets up zap test logging, intercepts HTTP requests using gock, and creates
// a context with the zap logger embedded.
func ClientTestSetup(t *testing.T) (ctx context.Context, logger *zap.Logger, teardown func()) {
	logger, teardownLogging := TestLogger(t)

	gock.Intercept()
	gock.Observe(GockLogObserver(logger))

	ctx, cancel := context.WithCancel(context.Background())

	teardown = func() {
		cancel()
		gock.OffAll()
		gock.Observe(nil)
		teardownLogging()
	}

	return
}
