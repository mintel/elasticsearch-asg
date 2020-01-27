package cmd

import (
	"github.com/prometheus/client_golang/prometheus" // Prometheus metrics.
)

// Namespace is the namespace to be used for Prometheus
// metrics throughout elasticsearch-asg.
const Namespace = "elasticsearchasg"

func BuildPromFQName(subsystem, name string) string {
	return prometheus.BuildFQName(Namespace, subsystem, name)
}
