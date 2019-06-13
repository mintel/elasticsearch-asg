package main

import (
	"context"
	"net/http"

	"go.uber.org/zap"
	kingpin "gopkg.in/alecthomas/kingpin.v2"

	esasg "github.com/mintel/elasticsearch-asg"
	eshealth "github.com/mintel/elasticsearch-asg/pkg/es/health"
)

const defaultURL = "http://localhost:9200"

var (
	esURL    = kingpin.Arg("url", "Elasticsearch URL. Default: "+defaultURL).Default(defaultURL).URL()
	endpoint = kingpin.Flag("endpoint", "Endpoint to serve healthchecks at.").Default(":9201").URL()
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

	mux, _, err := eshealth.NewMetricsHandler(ctx, (*esURL).String())
	if err != nil {
		logger.Fatal("Error creating healthcheck handler", zap.Error(err))
	}
	logger.Info("Serving health and readiness checks")
	err = http.ListenAndServe((*endpoint).Host, mux)
	if err != nil {
		logger.Fatal("Error serving healthchecks", zap.Error(err))
	}
}
