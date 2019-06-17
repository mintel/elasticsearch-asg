package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/heptiolabs/healthcheck"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	kingpin "gopkg.in/alecthomas/kingpin.v2"

	esasg "github.com/mintel/elasticsearch-asg"
	eshealth "github.com/mintel/elasticsearch-asg/pkg/es/health"
)

const defaultURL = "http://localhost:9200"

var (
	esURL     = kingpin.Arg("url", "Elasticsearch URL. Default: "+defaultURL).Default(defaultURL).URL()
	port      = kingpin.Flag("port", "Port to serve healthchecks on.").Default("9201").Int()
	namespace = kingpin.Flag("namespace", "Namespace to use for Prometheus metrics.").Default("elasticsearch").String()

	once                       = kingpin.Flag("once", "Execute checks once and exit with status code.").Bool()
	disableCheckHead           = kingpin.Flag("no-check-head", "Disable HEAD check.").Bool()
	disableCheckJoined         = kingpin.Flag("no-check-joined-cluster", "Disable joined cluster check.").Bool()
	disableCheckRollingUpgrade = kingpin.Flag("no-check-rolling-upgrade", "Disable rolling upgrade check.").Bool()
)

func main() {
	kingpin.CommandLine.Help = "Handle AWS Autoscaling Group Lifecycle hook events for Elasticsearch from an SQS queue."
	kingpin.Parse()

	logger := esasg.SetupLogging()
	defer func() {
		err := logger.Sync()
		if err != nil {
			panic(err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	atLeastOne := false
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

	if *once {
		logger.Info("Running checks once")
		w := httptest.NewRecorder()
		r, err := http.NewRequest("GET", "/ready", nil)
		if err != nil {
			logger.Panic("Failed to create request", zap.Error(err))
		}
		checks.ReadyEndpoint(w, r)
		if w.Result().StatusCode == 200 {
			logger.Info("Checks succeeded")
			os.Exit(0)
		}
		logger.Info("Checks failed")
		os.Exit(1)
	}

	logger.Info("Serving health and readiness checks")
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{}))
	mux.HandleFunc("/live", checks.LiveEndpoint)
	mux.HandleFunc("/ready", checks.ReadyEndpoint)
	endpoint := fmt.Sprintf(":%d", *port)
	if err := http.ListenAndServe(endpoint, mux); err != nil {
		logger.Fatal("Error serving healthchecks", zap.Error(err))
	}
}
