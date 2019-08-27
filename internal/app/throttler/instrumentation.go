package throttler

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Instrumentation holds Prometheus metrics specific to
// the throttler App.
type Instrumentation struct {
	// Count of the number of times throttler has
	// polled Elasticsearch for status information.
	Loops prometheus.Counter

	// Set to 1 if autoscaling of the Elasticsearch
	// cluster is enabled.
	ScalingStatus *prometheus.GaugeVec
}

// NewInstrumentation returns a new Instrumentation.
func NewInstrumentation(namespace string) *Instrumentation {
	return &Instrumentation{
		Loops: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "loops_total",
			Help:      "Count of the number of times throttler has polled Elasticsearch for status information.",
		}),
		ScalingStatus: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "autoscaling_enabled",
			Help:      "Set to 1 if autoscaling of the Elasticsearch cluster is enabled.",
		}, []string{"group"}),
	}
}

// Describe implements the prometheus.Collector interface.
func (m *Instrumentation) Describe(c chan<- *prometheus.Desc) {
	m.Loops.Describe(c)
	m.ScalingStatus.Describe(c)
}

// Collect implements the prometheus.Collector interface.
func (m *Instrumentation) Collect(c chan<- prometheus.Metric) {
	m.Loops.Collect(c)
	m.ScalingStatus.Collect(c)
}
