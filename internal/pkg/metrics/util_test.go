package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

// assertMetrics asserts that a prometheus.Gatherer has an expected
// number of metrics.
func assertMetrics(t *testing.T, r prometheus.Gatherer, expected int) {
	var count int
	metricFamilies, err := r.Gather()
	if !assert.NoError(t, err, "error while gathering metric families") {
		return
	}
	for _, mf := range metricFamilies {
		count += len(mf.Metric)
		t.Log(mf.GetName(), "-", mf.GetHelp())
		for _, m := range mf.Metric {
			t.Log(m.String())
		}
	}
	assert.Equal(t, expected, count, "wrong number of metrics")
}
