package snapshooter

import (
	"github.com/mintel/healthcheck"                  // Healthchecks framework.
	"github.com/pkg/errors"                          // Wrap errors with stacktrace.
	"github.com/prometheus/client_golang/prometheus" // Prometheus metrics.
)

type Healthchecks struct {
	Handler healthcheck.Handler

	// Flag to be set true once a connection
	// to Elasticsearch is successfully established.
	ElasticSessionCreated bool
}

func NewHealthchecks(r prometheus.Registerer, namespace string) *Healthchecks {
	h := &Healthchecks{
		Handler: healthcheck.NewMetricsHandler(
			r,
			namespace,
		),
	}

	// Add a liveness check that always succeeds just to show we're alive.
	h.Handler.AddLivenessCheck("alive", func() error { return nil })

	h.Handler.AddReadinessCheck("elasticsearch-session", func() error {
		if !h.ElasticSessionCreated {
			return errors.New("Elasticsearch session not yet ready")
		}
		return nil
	})

	return h
}
