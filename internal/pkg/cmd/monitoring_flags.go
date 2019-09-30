package cmd

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/mattn/go-isatty"    // Check if running in a terminal.
	"github.com/mintel/healthcheck" // Healthchecks framework.
	"go.uber.org/zap"               // Logging.
	"go.uber.org/zap/zapcore"

	"github.com/prometheus/client_golang/prometheus" // Prometheus metrics.
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MonitoringFlags represents a set of flags for
// setting up logging, healthchecks, and Prometheus metrics.
type MonitoringFlags struct {
	LogLevel    zapcore.Level // Logging level.
	Port        uint16        // Port to serve health checks, Prometheus metrics, and anything else on.
	LivePath    string        // HTTP path to serve the liveness healthcheck at.
	ReadyPath   string        // HTTP path to serve the readiness healthcheck at.
	MetricsPath string        // HTTP path to serve Prometheus metrics at.
}

// NewMonitoringFlags returns a new BaseFlags.
func NewMonitoringFlags(app Flagger, port int, logLevel string) *MonitoringFlags {
	var f MonitoringFlags

	app.Flag("log.level", "Set logging level.").
		HintOptions(
			zap.DebugLevel.CapitalString(),
			zap.DebugLevel.String(),
			zap.InfoLevel.CapitalString(),
			zap.InfoLevel.String(),
			zap.WarnLevel.CapitalString(),
			zap.WarnLevel.String(),
			zap.ErrorLevel.CapitalString(),
			zap.ErrorLevel.String(),
			zap.DPanicLevel.CapitalString(),
			zap.DPanicLevel.String(),
			zap.PanicLevel.CapitalString(),
			zap.PanicLevel.String(),
			zap.FatalLevel.CapitalString(),
			zap.FatalLevel.String(),
		).
		Default(logLevel).
		SetValue(&f.LogLevel)

	app.Flag("serve.port", "Port on which to expose health checks and Prometheus metrics.").
		Default(strconv.Itoa(port)).
		Uint16Var(&f.Port)

	app.Flag("serve.metrics", "Path at which to serve Prometheus metrics.").
		Default("/metrics").
		StringVar(&f.MetricsPath)

	app.Flag("serve.live", "Path at which to liveness healthcheck.").
		Default("/livez").
		StringVar(&f.LivePath)

	app.Flag("serve.ready", "Path at which to serve Prometheus metrics.").
		Default("/readyz").
		StringVar(&f.ReadyPath)

	return &f
}

// Logger returns a new logger based on the LogLevel flag.
func (f *MonitoringFlags) Logger() *zap.Logger {
	var conf zap.Config

	// If program is running in a terminal, use the zap default
	// dev logging config, else prod logging config.
	if isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		conf = zap.NewDevelopmentConfig()
	} else {
		conf = zap.NewProductionConfig()
	}

	conf.Level.SetLevel(f.LogLevel)

	logger, err := conf.Build()
	if err != nil {
		panic(fmt.Sprintf("error building logger: %s", err))
	}

	return logger
}

// ConfigureMux sets a mux to serve healthchecks and Prometheus metrics
// based on the path flags in f.
func (f *MonitoringFlags) ConfigureMux(mux *http.ServeMux, h healthcheck.Handler, g prometheus.Gatherer) *http.ServeMux {
	mux.Handle(f.MetricsPath, promhttp.HandlerFor(g, promhttp.HandlerOpts{}))
	mux.HandleFunc(f.LivePath, h.LiveEndpoint)
	mux.HandleFunc(f.ReadyPath, h.ReadyEndpoint)
	return mux
}

// Server returns a new HTTP server configured to listen on the
// port defined by the Port flag.
func (f *MonitoringFlags) Server(h http.Handler) *http.Server {
	return &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", f.Port),
		Handler: h,
	}
}
