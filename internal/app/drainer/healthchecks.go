package drainer

import (
	"github.com/mintel/healthcheck"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

type Healthchecks struct {
	Handler healthcheck.Handler

	// Flag to be set true once a connection
	// to Elasticsearch is successfully established.
	ElasticSessionCreated bool

	// Flag to be set true once an AWS session
	// has been successfully created.
	AWSSessionCreated bool
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

	h.Handler.AddReadinessCheck("aws-session", func() error {
		if !h.AWSSessionCreated {
			return errors.New("AWS session not yet ready")
		}
		return nil
	})

	return h
}
