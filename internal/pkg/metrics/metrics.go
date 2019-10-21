// Package metrics hold constants and utilities for instrumenting elasticsearch-asg
// with Prometheus metrics.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus" // Prometheus metrics.
)

// DefaultObjectives are default objectives for Prometheus Summary metrics.
var DefaultObjectives = map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001}

// MustRegisterOnce registers a set of Prometheus Collectors with the
// default Registerer, ignoring AlreadyRegisteredErrors. Other errors
// cause a panic.
func MustRegisterOnce(cs ...prometheus.Collector) {
	for _, c := range cs {
		if err := prometheus.Register(c); err != nil {
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				panic(err)
			}
		}
	}
}
