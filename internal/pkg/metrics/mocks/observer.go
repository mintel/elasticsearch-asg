package mocks

import (
	"github.com/prometheus/client_golang/prometheus" // Prometheus metrics.
	"github.com/stretchr/testify/mock"               // Mocking for tests.
)

// Observer is a mock type for the prometheus.Observer type.
type Observer struct {
	prometheus.Observer
	mock.Mock
}

func (m *Observer) Observe(f float64) {
	m.Called(f)
}
