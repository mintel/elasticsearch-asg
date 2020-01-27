package mocks

import (
	"github.com/prometheus/client_golang/prometheus" // Prometheus metrics.
	"github.com/stretchr/testify/mock"               // Mocking for tests.
)

// ObserverVec is a  mock type for the prometheus.ObserverVec type.
type ObserverVec struct {
	prometheus.ObserverVec
	mock.Mock
}

func (m *ObserverVec) With(lbls prometheus.Labels) prometheus.Observer {
	ret := m.Called(lbls)
	return ret.Get(0).(prometheus.Observer)
}
