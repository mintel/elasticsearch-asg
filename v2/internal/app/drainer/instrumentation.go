package drainer

import (
	"github.com/prometheus/client_golang/prometheus" // Prometheus metrics.
)

// Instrumentation holds Prometheus metrics specific to
// the drainer App.
type Instrumentation struct {
	// Count of the number of times cloudwatcher has
	// polled Elasticsearch for information.
	PollTotal prometheus.Counter

	// Total number of SQS messages received.
	MessagesReceived prometheus.Counter

	// Total number of Elasticsearch nodes that
	// got Spot interrupted.
	SpotInterruptions prometheus.Counter

	// Total number of Elasticsearch nodes that
	// have been terminated by an AutoScaling Group
	// scaling down.
	TerminationHookActionsTotal prometheus.Counter

	// Number of ongoing Elasticsearch node terminations
	// due to an AutoScaling Group scaledown.
	TerminationHookActionsInProgress prometheus.Gauge
}

// NewInstrumentation returns a new Metrics.
func NewInstrumentation(namespace string) *Instrumentation {
	return &Instrumentation{
		PollTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "polled_elasticsearch_total",
			Help:      "Count of the number of times drainer has polled Elasticsearch for status information.",
		}),
		MessagesReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "sqs_messages_received_total",
			Help:      "Total number of SQS messages received.",
		}),
		SpotInterruptions: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "spot_interruptions_total",
			Help:      "Total number of spot instance interruptions acted upon.",
		}),
		TerminationHookActionsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "autoscaling_terminations_total",
			Help:      "Total number of AutoScaling Group termination events received.",
		}),
		TerminationHookActionsInProgress: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "autoscaling_terminations_in_progress",
			Help:      "Number of AutoScaling Group terminations being handled.",
		}),
	}
}

// Describe implements the prometheus.Collector interface.
func (m *Instrumentation) Describe(c chan<- *prometheus.Desc) {
	m.PollTotal.Describe(c)
	m.MessagesReceived.Describe(c)
	m.SpotInterruptions.Describe(c)
	m.TerminationHookActionsTotal.Describe(c)
	m.TerminationHookActionsInProgress.Describe(c)
}

// Collect implements the prometheus.Collector interface.
func (m *Instrumentation) Collect(c chan<- prometheus.Metric) {
	m.PollTotal.Collect(c)
	m.MessagesReceived.Collect(c)
	m.SpotInterruptions.Collect(c)
	m.TerminationHookActionsTotal.Collect(c)
	m.TerminationHookActionsInProgress.Collect(c)
}
