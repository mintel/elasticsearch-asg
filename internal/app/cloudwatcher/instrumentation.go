package cloudwatcher

import (
	"github.com/dgraph-io/ristretto"                 // Cache.
	"github.com/prometheus/client_golang/prometheus" // Prometheus metrics.
)

// Instrumentation holds Prometheus metrics specific to
// the cloudwatcher App.
type Instrumentation struct {
	// Count of the number of times cloudwatcher has
	// polled Elasticsearch for information.
	Loops prometheus.Counter

	// Total number of metrics pushed to CloudWatch.
	MetricsPushed prometheus.Counter

	// Hit ratio of the EC2Instances cache.
	EC2InstancesCacheHitRatio prometheus.GaugeFunc
}

// NewInstrumentation returns a new Metrics.
func NewInstrumentation(namespace string, ec2InstancesCache *ristretto.Cache) *Instrumentation {
	return &Instrumentation{
		Loops: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "loops_total",
			Help:      "Count of the number of times cloudwatcher has polled Elasticsearch for status information.",
		}),
		MetricsPushed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "metrics_pushed_total",
			Help:      "Total number of metrics pushed to CloudWatch.",
		}),
		EC2InstancesCacheHitRatio: prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "vcpu_cache_hit_ratio",
			Help:      "The hit ratio of the cache of EC2 instances.",
		}, func() float64 { return ec2InstancesCache.Metrics().Ratio() }),
	}
}

// Describe implements the prometheus.Collector interface.
func (m *Instrumentation) Describe(c chan<- *prometheus.Desc) {
	m.Loops.Describe(c)
	m.MetricsPushed.Describe(c)
	m.EC2InstancesCacheHitRatio.Describe(c)
}

// Collect implements the prometheus.Collector interface.
func (m *Instrumentation) Collect(c chan<- prometheus.Metric) {
	m.Loops.Collect(c)
	m.MetricsPushed.Collect(c)
	m.EC2InstancesCacheHitRatio.Collect(c)
}
