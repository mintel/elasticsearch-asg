// Package metrics hold constants and utilities for instrumenting elasticsearch-asg
// with Prometheus metrics.
package metrics

// Namespace is the namespace to be used for Prometheus metrics throughout elasticsearch-asg.
const Namespace = "elasticsearchasg"

const (
	// LabelMethod is the Prometheus label name for HTTP method.
	LabelMethod = "method"

	// LabelStatusCode is the Prometheus label name for HTTP status codes.
	LabelStatusCode = "code"

	// LabelStatus is the Prometheus label name for the status of a process
	// such as "success" or "error".
	LabelStatus = "status"
)

// DefaultObjectives are default objectives for Prometheus Summary metrics.
var DefaultObjectives = map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001}
