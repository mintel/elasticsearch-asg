package main

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/heptiolabs/healthcheck"
	elastic "github.com/olivere/elastic/v7"  // Elasticsearch client
	"go.uber.org/zap"                        // Logging
	kingpin "gopkg.in/alecthomas/kingpin.v2" // Command line option parser

	// AWS clients and stuff.
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/mintel/elasticsearch-asg/internal/app/cloudwatcher"  // Implementation
	"github.com/mintel/elasticsearch-asg/internal/pkg/cmd"           // Common logging setup func
	"github.com/mintel/elasticsearch-asg/internal/pkg/elasticsearch" // Complex Elasticsearch services
	"github.com/mintel/elasticsearch-asg/internal/pkg/metrics"       // Prometheus metrics
)

// Request retry count/timeouts.
const (
	awsMaxRetries = 3
	esRetryInit   = 150 * time.Millisecond
	esRetryMax    = 1200 * time.Millisecond
)

// defaultURL is the default Elasticsearch URL.
const defaultURL = "http://localhost:9200"

var (
	// loopDuration tracks the duration of main loop of cloudwatcher.
	// It has a label `status` which is one of "success", "error", or "sleep".
	loopDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: cloudwatcher.Subsystem,
		Name:      "mainloop_duration_seconds",
		Help:      "Tracks the duration of main loop.",
		Buckets:   prometheus.DefBuckets, // TODO: Define better buckets.
	}, []string{metrics.LabelStatus})
	loopDurationSuccess = loopDuration.WithLabelValues("success")
	loopDurationError   = loopDuration.WithLabelValues("error")
	loopDurationSleep   = loopDuration.WithLabelValues("sleep")
)

// Command line opts
var (
	esURL         = kingpin.Arg("url", "Elasticsearch URL. Default: "+defaultURL).Default(defaultURL).URL()
	interval      = kingpin.Flag("interval", "Time between pushing metrics.").Default("1m").Duration()
	region        = kingpin.Flag("region", "AWS Region.").String()
	namespace     = kingpin.Flag("namespace", "AWS CloudWatch metrics namespace.").Default("Elasticsearch").String()
	metricsListen = kingpin.Flag("metrics.listen", "Address on which to expose Prometheus metrics.").Default(":9700").String()
	metricsPath   = kingpin.Flag("metrics.path", "Path under which to expose Prometheus metrics.").Default("/metrics").String()
)

func main() {
	kingpin.CommandLine.Help = "Push Elasticsearch metrics to AWS CloudWatch to run AWS Autoscaling Groups."
	kingpin.Parse()

	logger := cmd.SetupLogging()
	defer func() {
		// Make sure any buffered logs get flushed before exiting successfully.
		// This should never happen because cloudwatcher should never exit successfully,
		// but just in case...
		// Subsequent calls to loger.Fatal() perform their own Sync().
		// See: https://github.com/uber-go/zap/blob/master/FAQ.md#why-include-dedicated-panic-and-fatal-log-levels
		// Do this inside a closure func so that the linter will stop complaining
		// about not checking the error output of Sync().
		_ = logger.Sync()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Make AWS clients.
	sess := session.Must(session.NewSession())
	prometheus.WrapRegistererWithPrefix(
		prometheus.BuildFQName(metrics.Namespace, "", cloudwatcher.Subsystem),
		prometheus.DefaultRegisterer,
	).MustRegister(metrics.InstrumentAWS(&sess.Handlers, nil)...) // TODO: Define better buckets.
	awsConfig := aws.NewConfig().WithMaxRetries(awsMaxRetries)
	if region != nil && *region != "" {
		// If --region was set use that...
		awsConfig = awsConfig.WithRegion(*region)
	} else if region, _ := ec2metadata.New(sess).Region(); region != "" {
		// ... else try to get current region from EC2 Instance Metadata endpoint.
		awsConfig = awsConfig.WithRegion(region)
	}
	cwClient := cloudwatch.New(sess, awsConfig)
	ec2Client := ec2.New(sess, awsConfig)

	// Make Elasticsearch client.
	_, esQuery, err := elasticsearch.New(
		ctx,
		(*esURL).String(),
		elastic.SetRetrier(elastic.NewBackoffRetrier(elastic.NewExponentialBackoff(esRetryInit, esRetryMax))),
	)
	if err != nil {
		logger.Fatal("error creating Elasticsearch client", zap.Error(err))
	}

	// Setup healthchecks
	health := healthcheck.NewMetricsHandler(prometheus.DefaultRegisterer, prometheus.BuildFQName(metrics.Namespace, "", cloudwatcher.Subsystem))
	health.AddLivenessCheck("up", func() error {
		return nil
	})
	health.AddReadinessCheck("noerror", func() error {
		return err
	})

	// Serve health checks and Prometheus metrics.
	go func() {
		http.Handle(*metricsPath, promhttp.Handler())
		http.HandleFunc("/live", health.LiveEndpoint)
		http.HandleFunc("/ready", health.ReadyEndpoint)
		if err := http.ListenAndServe(*metricsListen, nil); err != nil {
			logger.Fatal("error serving metrics", zap.Error(err))
		}
	}()

	var nodes map[string]*elasticsearch.Node
	var vcpuCounts map[string]int
	var loopTimer *prometheus.Timer
	for range time.NewTicker(*interval).C { // Each --interval...
		if loopTimer != nil {
			loopTimer.ObserveDuration()
		}
		loopTimer = prometheus.NewTimer(nil)

		// Get info about all the nodes in Elasticsearch.
		nodes, err = esQuery.Nodes(ctx)
		if err != nil {
			logger.Error("error getting Elasticsearch nodes info", zap.Error(err))
			loopDurationError.Observe(loopTimer.ObserveDuration().Seconds())
			continue
		}

		// Get a count of vCPUs per instance. This is use when calculating Load %.
		instanceIDs := make([]string, 0, len(nodes))
		for id := range nodes {
			instanceIDs = append(instanceIDs, id)
		}
		vcpuCounts, err = cloudwatcher.GetInstanceVCPUCount(ctx, ec2Client, instanceIDs)
		if err != nil {
			logger.Error("error getting EC2 instances vCPU counts", zap.Error(err))
			loopDurationError.Observe(loopTimer.ObserveDuration().Seconds())
			continue
		}

		// Generate CloudWatch metric data points from nodes and vcpu counts.
		metricData := cloudwatcher.MakeCloudwatchData(nodes, vcpuCounts)
		for _, datum := range metricData {
			cloudwatcher.LogDatum(logger, datum)
		}
		if err = cloudwatcher.PushCloudwatchData(ctx, cwClient, *namespace, metricData); err != nil {
			logger.Error("error pushing metrics to CloudWatch", zap.Error(err))
			loopDurationError.Observe(loopTimer.ObserveDuration().Seconds())
			continue
		}

		loopDurationSuccess.Observe(loopTimer.ObserveDuration().Seconds())
		loopTimer = prometheus.NewTimer(loopDurationSleep)
	}
}
