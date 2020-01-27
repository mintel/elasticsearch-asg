package cmd

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/mintel/healthcheck"                  // Healthchecks framework.
	"github.com/prometheus/client_golang/prometheus" // Prometheus metrics.
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ServerFlags represents a set of flags for setting up
// a server with healthchecks and Prometheus metrics.
type ServerFlags struct {
	Port        uint16 // Port to serve health checks, Prometheus metrics, and anything else on.
	LivePath    string // HTTP path to serve the liveness healthcheck at.
	ReadyPath   string // HTTP path to serve the readiness healthcheck at.
	MetricsPath string // HTTP path to serve Prometheus metrics at.
}

// NewServerFlags returns a new ServerFlags.
func NewServerFlags(app Flagger, port int) *ServerFlags {
	var f ServerFlags

	app.Flag("serve.port", "Port on which to expose healthchecks and Prometheus metrics.").
		Default(strconv.Itoa(port)).
		Uint16Var(&f.Port)

	app.Flag("serve.metrics", "Path at which to serve Prometheus metrics.").
		Default("/metrics").
		StringVar(&f.MetricsPath)

	app.Flag("serve.live", "Path at which to serve liveness healthcheck.").
		Default("/livez").
		StringVar(&f.LivePath)

	app.Flag("serve.ready", "Path at which to serve readiness healthcheck.").
		Default("/readyz").
		StringVar(&f.ReadyPath)

	return &f
}

// ConfigureMux sets a mux to serve healthchecks and Prometheus metrics
// based on the path flags in f.
func (f *ServerFlags) ConfigureMux(mux *http.ServeMux, h healthcheck.Handler, g prometheus.Gatherer) *http.ServeMux {
	mux.Handle(f.MetricsPath, promhttp.HandlerFor(g, promhttp.HandlerOpts{}))
	mux.HandleFunc(f.LivePath, h.LiveEndpoint)
	mux.HandleFunc(f.ReadyPath, h.ReadyEndpoint)
	return mux
}

// NewServer returns a new HTTP server configured to listen on the
// port defined by the Port flag.
func (f *ServerFlags) NewServer(h http.Handler) *http.Server {
	return &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", f.Port),
		Handler: h,
	}
}
