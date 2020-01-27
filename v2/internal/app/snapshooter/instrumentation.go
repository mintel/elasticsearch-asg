package snapshooter

import (
	"github.com/prometheus/client_golang/prometheus" // Prometheus metrics.

	"github.com/mintel/elasticsearch-asg/v2/internal/pkg/metrics" // Prometheus metrics tools.
)

// Instrumentation holds Prometheus metrics specific to
// the snapshooter App.
type Instrumentation struct {
	// Count of the number of times snapshooter has
	// polled Elasticsearch for information.
	Loops prometheus.Counter

	// Number of seconds spent sleeping.
	SleepSeconds prometheus.Summary

	// Number of seconds spent creating snapshots.
	SnapshotCreationSeconds prometheus.Summary

	// Number of seconds spent deleting snapshots.
	SnapshotDeletionSeconds prometheus.Summary

	// A count of Elasticsearch snapshots created.
	SnapshotsCreated prometheus.Counter

	// A count of Elasticsearch snapshots deleted.
	SnapshotsDeleted prometheus.Counter

	// A count of Elasticsearch snapshots skipped
	// because a previous operation was still running.
	SnapshotsSkipped prometheus.Counter

	// Number of snapshots in Elasticsearch.
	Snapshots prometheus.Gauge
}

// NewInstrumentation returns a new Metrics.
func NewInstrumentation(namespace string) *Instrumentation {
	return &Instrumentation{
		Loops: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "loops_total",
			Help:      "Count of the number of times snapshooter has polled Elasticsearch for status information.",
		}),
		SleepSeconds: prometheus.NewSummary(prometheus.SummaryOpts{
			Namespace:  namespace,
			Name:       "sleep_duration_seconds",
			Help:       "Number of seconds spent sleeping.",
			Objectives: metrics.DefaultObjectives, // TODO: Define better objectives.
		}),
		SnapshotCreationSeconds: prometheus.NewSummary(prometheus.SummaryOpts{
			Namespace:  namespace,
			Name:       "snapshot_creation_duration_seconds",
			Help:       "Number of seconds spent creating snapshots.",
			Objectives: metrics.DefaultObjectives, // TODO: Define better objectives.
		}),
		SnapshotDeletionSeconds: prometheus.NewSummary(prometheus.SummaryOpts{
			Namespace:  namespace,
			Name:       "snapshot_deletion_duration_seconds",
			Help:       "Number of seconds spent deleting snapshots.",
			Objectives: metrics.DefaultObjectives, // TODO: Define better objectives.
		}),
		SnapshotsCreated: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "snapshots_created_total",
			Help:      "Count of Elasticsearch snapshots created.",
		}),
		SnapshotsDeleted: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "snapshots_deleted_total",
			Help:      "Count of Elasticsearch snapshots deleted.",
		}),
		SnapshotsSkipped: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "snapshots_skipped_total",
			Help:      "Count of Elasticsearch snapshots skipped because previous operations were still running.",
		}),
		Snapshots: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "snapshots",
			Help:      "The number of snapshots in Elasticsearch.",
		}),
	}
}

// Describe implements the prometheus.Collector interface.
func (m *Instrumentation) Describe(c chan<- *prometheus.Desc) {
	m.Loops.Describe(c)
	m.SleepSeconds.Describe(c)
	m.SnapshotCreationSeconds.Describe(c)
	m.SnapshotDeletionSeconds.Describe(c)
	m.SnapshotsCreated.Describe(c)
	m.SnapshotsDeleted.Describe(c)
	m.SnapshotsSkipped.Describe(c)
	m.Snapshots.Describe(c)
}

// Collect implements the prometheus.Collector interface.
func (m *Instrumentation) Collect(c chan<- prometheus.Metric) {
	m.Loops.Collect(c)
	m.SleepSeconds.Collect(c)
	m.SnapshotCreationSeconds.Collect(c)
	m.SnapshotDeletionSeconds.Collect(c)
	m.SnapshotsCreated.Collect(c)
	m.SnapshotsDeleted.Collect(c)
	m.SnapshotsSkipped.Collect(c)
	m.Snapshots.Collect(c)
}
