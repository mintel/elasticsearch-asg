package health

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/heptiolabs/healthcheck"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/mintel/elasticsearch-asg/mocks/mockhttp"
)

func setup(t *testing.T, checkFactory func(context.Context, string) healthcheck.Check) (healthcheck.Check, *httptest.Server, *mockhttp.Mux, func()) {
	logger := zaptest.NewLogger(t)
	defer func() {
		if err := logger.Sync(); err != nil {
			panic(err)
		}
	}()
	t1 := zap.ReplaceGlobals(logger)
	t2 := zap.RedirectStdLog(logger)

	ctx, cancel := context.WithCancel(context.Background())
	mux := &mockhttp.Mux{}
	server := httptest.NewServer(mux)
	check := checkFactory(ctx, server.URL)
	return check, server, mux, func() {
		cancel()
		server.Close()
		t2()
		t1()
		if err := logger.Sync(); err != nil {
			panic(err)
		}
	}
}
