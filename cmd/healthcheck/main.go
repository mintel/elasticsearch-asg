package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/prometheus/client_golang/prometheus"          // Prometheus
	"github.com/prometheus/client_golang/prometheus/promhttp" // Prometheus HTTP handler.

	"github.com/heptiolabs/healthcheck"      // Framework for implementing healthchecks.
	"go.uber.org/zap"                        // Logging
	kingpin "gopkg.in/alecthomas/kingpin.v2" // Command line args parser

	"github.com/mintel/elasticsearch-asg/internal/pkg/cmd"       // Common logging setup func
	eshealth "github.com/mintel/elasticsearch-asg/pkg/es/health" // Funcs to evaluate Elasticsearch health in various ways
)

// defaultURL is the default Elasticsearch URL.
const defaultURL = "http://localhost:9200"

var (
	esURL     = kingpin.Arg("url", "Elasticsearch URL. Default: "+defaultURL).Default(defaultURL).URL()
	port      = kingpin.Flag("port", "Port to serve healthchecks on.").Default("9201").Int()
	namespace = kingpin.Flag("namespace", "Namespace to use for Prometheus metrics.").Default("elasticsearch").String()
	once      = kingpin.Flag("once", "Execute checks once and exit with status code.").Bool()

	// Allow various checks to be disabled.
	disableCheckHead           = kingpin.Flag("no-check-head", "Disable HEAD check.").Bool()
	disableCheckJoined         = kingpin.Flag("no-check-joined-cluster", "Disable joined cluster check.").Bool()
	disableCheckRollingUpgrade = kingpin.Flag("no-check-rolling-upgrade", "Disable rolling upgrade check.").Bool()
)

func main() {
	kingpin.CommandLine.Help = "Handle AWS Autoscaling Group Lifecycle hook events for Elasticsearch from an SQS queue."
	kingpin.Parse()

	logger := cmd.SetupLogging()
	defer func() {
		defer func() {
			// Make sure any buffered logs get flushed before exiting successfully.
			// This might happen if the --once flag is set.
			// Subsequent calls to loger.Fatal() perform their own Sync().
			// See: https://github.com/uber-go/zap/blob/master/FAQ.md#why-include-dedicated-panic-and-fatal-log-levels
			// Do this inside a closure func so that the linter will stop complaining
			// about not checking the error output of Sync().
			_ = logger.Sync()
		}()
	}()

	ctx := context.Background()

	// Create a HTTP handler that runs the healthchecks.
	atLeastOne := false // At least one healthcheck is enabled.
	checks := healthcheck.NewMetricsHandler(prometheus.DefaultRegisterer, *namespace)
	if !*disableCheckHead {
		atLeastOne = true
		checks.AddLivenessCheck("HEAD", eshealth.CheckLiveHEAD(ctx, (*esURL).String()))
	}
	if !*disableCheckJoined {
		atLeastOne = true
		checks.AddReadinessCheck("joined-cluster", eshealth.CheckReadyJoinedCluster(ctx, (*esURL).String()))
	}
	if !*disableCheckRollingUpgrade {
		atLeastOne = true
		checks.AddReadinessCheck("rolling-upgrade", eshealth.CheckReadyRollingUpgrade(ctx, (*esURL).String()))
	}
	if !atLeastOne {
		logger.Fatal("No health checks enabled")
	}

	// If the --once flag is set, artifically call the healthcheck HTTP handler,
	// and return an exit code based on the result.
	if *once {
		logger.Info("Running checks once")
		w := httptest.NewRecorder()
		r, err := http.NewRequest("GET", "/ready", nil)
		if err != nil {
			logger.Panic("Failed to create request", zap.Error(err))
		}
		checks.ReadyEndpoint(w, r) // Calling ReadyEndpoint also calls LiveEndpoint (first).
		if w.Result().StatusCode == 200 {
			logger.Info("Checks succeeded")
			os.Exit(0)
		}
		logger.Info("Checks failed")
		os.Exit(1)
	}

	logger.Info("Serving health and readiness checks")
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/live", checks.LiveEndpoint)
	mux.HandleFunc("/ready", checks.ReadyEndpoint)
	endpoint := fmt.Sprintf(":%d", *port)
	if err := http.ListenAndServe(endpoint, mux); err != nil {
		logger.Fatal("Error serving healthchecks", zap.Error(err))
	}
}
