package healthchecker

import (
	"github.com/mintel/healthcheck"                  // Healthchecks framework.
	"github.com/pkg/errors"                          // Wrap errors with stacktrace.
	"github.com/prometheus/client_golang/prometheus" // Prometheus metrics.
)

type Healthchecks struct {
	Handler healthcheck.Handler

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
	h.Handler.AddLivenessCheck("healthchecker-alive", func() error { return nil })

	h.Handler.AddReadinessCheck("healthchecker-elasticsearch-session", func() error {
		if !h.ElasticSessionCreated {
			return errors.New("Elasticsearch session not yet ready")
		}
		return nil
	})

	return h
}
