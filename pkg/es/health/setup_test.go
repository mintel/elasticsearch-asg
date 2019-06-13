package health

import (
	"net/http"
	"net/http/httptest"
	"testing"

	elastic "github.com/olivere/elastic/v7"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func setup(t *testing.T) (*elastic.Client, *httptest.Server, *http.ServeMux, func()) {
	logger := zaptest.NewLogger(t)
	defer func() {
		if err := logger.Sync(); err != nil {
			panic(err)
		}
	}()
	t1 := zap.ReplaceGlobals(logger)
	t2 := zap.RedirectStdLog(logger)

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	lc := &lazyClient{URL: server.URL}
	es, err := lc.Client()
	if err != nil {
		panic(err)
	}
	return es, server, mux, func() {
		es.Stop()
		server.Close()
		t2()
		t1()
		if err := logger.Sync(); err != nil {
			panic(err)
		}
	}
}
