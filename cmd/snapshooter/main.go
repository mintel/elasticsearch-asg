package main

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/heptiolabs/healthcheck"
	"go.uber.org/zap"                        // Logging
	kingpin "gopkg.in/alecthomas/kingpin.v2" // Command line args parser

	"github.com/mintel/elasticsearch-asg/internal/app/snapshooter"   // Implementation
	"github.com/mintel/elasticsearch-asg/internal/pkg/cmd"           // Common logging setup func
	"github.com/mintel/elasticsearch-asg/internal/pkg/elasticsearch" // Elasticsearch Command/Query service
	"github.com/mintel/elasticsearch-asg/internal/pkg/metrics"       // Prometheus metrics
)

var (
	// loopDuration tracks the duration of main loop of snapshooter.
	// It has a label `status` which is one of "success", "error", or "sleep".
	// Alerts should be implemented to make sure the duration of sleep does
	// not approach zero, as this would indicate that the process of creating/deleting
	// snapshots is taking too long.
	loopDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: snapshooter.Subsystem,
		Name:      "mainloop_duration_seconds",
		Help:      "Tracks the duration of main loop.",
		Buckets:   prometheus.DefBuckets, // TODO: Define better buckets.
	}, []string{metrics.LabelStatus})
	loopDurationSuccess = loopDuration.WithLabelValues("success")
	loopDurationError   = loopDuration.WithLabelValues("error")
	loopDurationSleep   = loopDuration.WithLabelValues("sleep")
)

// defaultURL is the default Elasticsearch URL.
const defaultURL = "http://localhost:9200"

// Command line opts
var (
	esURL         = kingpin.Arg("url", "Elasticsearch URL. Default: "+defaultURL).Default(defaultURL).URL()
	windows       = kingpin.Flag("window", "Snapshot frequency + TTL. May be set multiple times. ISO 8601 Duration string format. Example: `--window P1M=PT1H` == keep hourly snapshots for 1 month.").PlaceHolder("P1M=PT1H").Required().StringMap()
	delete        = kingpin.Flag("delete", "If set, clean up old snapshots. This is false by default for safety's sake.").Short('d').Bool()
	repoName      = kingpin.Flag("repo", "Name of the snapshot repository.").Default("backups").String()
	repoType      = kingpin.Flag("type", "If set, create a repository of this type before creating snapshots. See also: '--settings'").String()
	repoSettings  = kingpin.Flag("settings", "Use these settings creating the snapshot repository. May be set multiple times. Example: `--type=s3 --settings bucket=my_bucket`").StringMap()
	metricsListen = kingpin.Flag("metrics.listen", "Address on which to expose Prometheus metrics.").Default(":9702").String()
	metricsPath   = kingpin.Flag("metrics.path", "Path under which to expose Prometheus metrics.").Default("/metrics").String()
)

var logger *zap.Logger // XXX: I don't like a global logger var like this. Refactor to derive logger from context.

func main() {
	kingpin.CommandLine.Help = "Create and clean up Elasticsearch snapshots on a schedule."
	kingpin.Parse()

	// Set up logger.
	logger = cmd.SetupLogging().With(zap.String("snapshot_repository", *repoName))
	defer func() {
		// Make sure any buffered logs get flushed before exiting successfully.
		// This should never happen because snapshooter should never exit successfully,
		// but just in case...
		// Subsequent calls to loger.Fatal() perform their own Sync().
		// See: https://github.com/uber-go/zap/blob/master/FAQ.md#why-include-dedicated-panic-and-fatal-log-levels
		// Do this inside a closure func so that the linter will stop complaining
		// about not checking the error output of Sync().
		_ = logger.Sync()
	}()

	// Parse the snapshot schedule.
	snapshotSchedule := make(snapshooter.SnapshotWindows, 0)
	for keepFor, every := range *windows {
		w, err := snapshooter.NewSnapshotWindow(every, keepFor)
		if err != nil {
			logger.Fatal("error parsing snapshot window",
				zap.String("keepFor", keepFor),
				zap.String("every", every),
				zap.Error(err),
			)
		}
		snapshotSchedule = append(snapshotSchedule, w)
	}

	ctx := context.Background()

	// Craete Elasticsearch client.
	esCmd, esQuery, err := elasticsearch.New(ctx, (*esURL).String())
	if err != nil {
		logger.Fatal("error creating Elasticsearch client", zap.Error(err))
	}

	// If --type/--settings flags are set, create the snapshot repository if it doesn't exist.
	if repoType != nil && *repoType != "" {
		if err := esCmd.EnsureSnapshotRepo(ctx, *repoName, *repoType, *repoSettings); err != nil {
			logger.Fatal("error ensuring snapshot repository exists", zap.Error(err))
		}
	}

	// Setup healthchecks
	health := healthcheck.NewMetricsHandler(prometheus.DefaultRegisterer, prometheus.BuildFQName(metrics.Namespace, "", snapshooter.Subsystem))
	health.AddLivenessCheck("up", func() error {
		return nil
	})

	// Serve health checks and Prometheus metrics.
	go func() {
		http.Handle(*metricsPath, promhttp.Handler())
		http.HandleFunc("/live", health.LiveEndpoint)
		if err := http.ListenAndServe(*metricsListen, nil); err != nil {
			logger.Fatal("error serving metrics", zap.Error(err))
		}
	}()

	for nextSnapshot := snapshotSchedule.Next(); ; nextSnapshot = snapshotSchedule.Next() {
		sleepTime := time.Until(nextSnapshot)
		time.Sleep(sleepTime) // Wait to start the snapshot
		loopDurationSleep.Observe(sleepTime.Seconds())

		// Start a goroutine to create/delete snapshots.
		// Accoring to https://www.elastic.co/guide/en/elasticsearch/reference/7.0/modules-snapshots.html
		//   Only one snapshot process can be executed in the cluster at any time.
		//   While snapshot of a particular shard is being created this shard cannot be moved to another node,
		//   which can interfere with rebalancing process and allocation filtering.
		//   Elasticsearch will only be able to move a shard to another node
		//   (according to the current allocation filtering settings and rebalancing algorithm)
		//   once the snapshot is finished.
		// If this goroutine doesn't finish by the time the next one is started,
		// Elasticsearch will probably return an error and snapshooter will exit.
		go func(t time.Time) {
			logger.Debug("starting snapshot create/delete goroutine")
			timer := prometheus.NewTimer(nil)
			if err := esCmd.CreateSnapshot(ctx, *repoName, snapshooter.SnapshotFormat, t); err != nil {
				loopDurationError.Observe(timer.ObserveDuration().Seconds())
				logger.Error("error while creating new snapshot", zap.Error(err))
				return
			}
			if !*delete {
				return // If the --delete flag isn't set, don't clean up old snapshots.
			}
			snapshots, err := esQuery.GetSnapshots(ctx, *repoName)
			if err != nil {
				loopDurationError.Observe(timer.ObserveDuration().Seconds())
				logger.Error("error getting existing snapshots", zap.Error(err))
				return
			}
			for _, s := range snapshots {
				t, err := time.Parse(snapshooter.SnapshotFormat, s.Snapshot)
				if err != nil {
					logger.Warn("error parsing time from snapshot name",
						zap.String("snapshot", s.Snapshot),
						zap.Error(err),
					)
					continue
				}
				if !snapshotSchedule.Keep(t) {
					logger.Info("deleting snapshot", zap.String("snapshot", s.Snapshot))
					if err := esCmd.DeleteSnapshot(ctx, *repoName, s.Snapshot); err != nil {
						loopDurationError.Observe(timer.ObserveDuration().Seconds())
						logger.Error("error deleting old snapshot",
							zap.String("snapshot", s.Snapshot),
							zap.Error(err),
						)
						return
					}
				}
			}
			loopDurationSuccess.Observe(timer.ObserveDuration().Seconds())
		}(nextSnapshot)
	}
}
