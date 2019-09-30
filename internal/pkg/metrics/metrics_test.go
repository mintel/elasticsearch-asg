package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus" // Prometheus metrics.
	"github.com/stretchr/testify/assert"             // Test assertions e.g. equality.
)

func TestMustRegisterOnce(t *testing.T) {
	r := prometheus.NewRegistry()
	c := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "foo",
		Subsystem: "bar",
		Name:      "baz",
		Help:      "Example counter",
	})

	originalR := prometheus.DefaultRegisterer
	defer func() {
		prometheus.DefaultRegisterer = originalR
	}()

	prometheus.DefaultRegisterer = r
	mfs, err := r.Gather()
	assert.NoError(t, err)
	assert.Empty(t, mfs)

	MustRegisterOnce(c)
	mfs, err = r.Gather()
	assert.NoError(t, err)
	if assert.Len(t, mfs, 1) {
		mf := mfs[0]
		assert.Equal(t, "Example counter", mf.GetHelp())
	}

	MustRegisterOnce(c)
	mfs, err = r.Gather()
	assert.NoError(t, err)
	if assert.Len(t, mfs, 1) {
		mf := mfs[0]
		assert.Equal(t, "Example counter", mf.GetHelp())
	}
}
