// Package metrics hold constants and utilities for instrumenting elasticsearch-asg
// with Prometheus metrics.
package metrics

// Namespace is the namespace to be used for Prometheus metrics throughout elasticsearch-asg.
const Namespace = "elasticsearchasg"

const (
	// LabelStatusCode is the Prometheus label name for HTTP status codes.
	LabelStatusCode = "status_code"
)

// DefaultObjectives are default objectives for Prometheus Summary metrics.
var DefaultObjectives = map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001}
