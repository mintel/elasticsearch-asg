package cmd

import (
	"github.com/mintel/healthcheck"
	"github.com/prometheus/client_golang/prometheus"
)

// NewHealthchecksHandler returns a new healthcheck.Handler, configured
// with a basic liveness check and Prometheus healthcheck status
// metrics for a given app name.
func NewHealthchecksHandler(r prometheus.Registerer, appName string) healthcheck.Handler {
	h := healthcheck.NewMetricsHandler(
		r,
		BuildPromFQName("", appName),
	)
	h.AddLivenessCheck("alive", func() error { return nil })
	return h
}
