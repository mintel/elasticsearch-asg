// Package health implements Elasticsearch healthchecks (using https://github.com/heptiolabs/healthcheck)
// to check the liveness and readiness of an Elasticsearch node.
package health

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/heptiolabs/healthcheck"
	elastic "github.com/olivere/elastic/v7"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

var (
	// liveChecks maps name to liveness healthchecks.
	liveChecks = make(map[string]func(context.Context, *lazyClient) error)

	// readyChecks maps name to readiness healthchecks.
	readyChecks = make(map[string]func(context.Context, *lazyClient) error)
)

// Healthcheck is the type for Elasticsearch healthcheck functions.
type Healthcheck func(context.Context, *elastic.Client, *zap.Logger) error

func registerCheck(name string, f Healthcheck, d map[string]func(context.Context, *lazyClient) error) {
	d[name] = func(ctx context.Context, lc *lazyClient) error {
		logger := zap.L().Named(name)
		client, err := lc.Client()
		if err != nil {
			logger.Error("Error creating Elasticsearch client", zap.Error(err))
			return err
		}
		return f(ctx, client, logger)
	}
}

// RegisterLiveCheck adds a liveness check to the global set of healthchecks.
func RegisterLiveCheck(name string, f Healthcheck) {
	registerCheck(name, f, liveChecks)
}

// RegisterReadyCheck adds a readiness check to the global set of healthchecks.
func RegisterReadyCheck(name string, f Healthcheck) {
	registerCheck(name, f, readyChecks)
}

// The cluster state endpoint tends to hang if the node hasn't joined a cluster.
// Create an http client with a timeout.
var (
	timeout   = 250 * time.Millisecond
	netClient = &http.Client{
		Timeout: timeout,
	}
)

type lazyClient struct {
	URL string

	mu     sync.Mutex
	client *elastic.Client
}

// Client returns an elastic client.
func (lc *lazyClient) Client() (*elastic.Client, error) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if lc.client == nil {
		client, err := elastic.NewClient(
			elastic.SetURL(lc.URL),
			elastic.SetHttpClient(netClient), // Use client with timeout
			elastic.SetSniff(false),          // We only want to check the health of one node.
			elastic.SetHealthcheck(false),    // We'll be doing the healthchecks, damn it!
		)
		if err != nil {
			return nil, err
		}
		lc.client = client
	}
	return lc.client, nil
}

// NewHandler returns an http Handler configured to test the liveness and readiness
// of an Elasticsearch node at URL.
func NewHandler(ctx context.Context, URL string) (healthcheck.Handler, error) {
	lc := &lazyClient{URL: URL}
	health := healthcheck.NewHandler()

	for name, check := range liveChecks {
		health.AddLivenessCheck(name, healthcheck.Check(func() error {
			return check(ctx, lc)
		}))
	}

	for name, check := range readyChecks {
		health.AddReadinessCheck(name, healthcheck.Check(func() error {
			return check(ctx, lc)
		}))
	}

	return health, nil
}

// NewMetricsHandler returns an *http.ServeMux that responds to healthcheck and Prometheus metrics requests.
// It also returns the healthcheck.Handler is case you want to add additional checks.
func NewMetricsHandler(ctx context.Context, URL string) (*http.ServeMux, healthcheck.Handler, error) {
	lc := &lazyClient{URL: URL}
	registry := prometheus.NewRegistry()
	health := healthcheck.NewMetricsHandler(registry, "elasticsearch")

	for name, check := range liveChecks {
		health.AddLivenessCheck(name, healthcheck.Check(func() error {
			return check(ctx, lc)
		}))
	}

	for name, check := range readyChecks {
		health.AddReadinessCheck(name, healthcheck.Check(func() error {
			return check(ctx, lc)
		}))
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/live", health.LiveEndpoint)
	mux.HandleFunc("/ready", health.ReadyEndpoint)
	return mux, health, nil
}
