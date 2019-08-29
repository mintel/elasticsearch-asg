package metrics

import (
	"strconv"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/prometheus/client_golang/prometheus"
)

// InstrumentAWS adds Prometheus metrics to an AWS client or session.
//
// A duration histogram is observed for each AWS API request.
// The histogram has labels for AWS service, operation, HTTP method,
// and returned HTTP status code. This fulfills the
//
// If buckets is nil, prometheus.DefBuckets is used.
//
// IF the AWS clients retry on error, each attempt counts as a separate sample.
//
// It is recommended to use this for debugging purposes only.
// Recording every combination of service, operation, method, and status code
// can result in a lot of metrics.
//
// Example:
//
//   sess := session.Must(session.NewSession())
//   durationCollectors := metrics.InstrumentAWS(&sess.Handlers, metrics.DefaultObjectives)
//   prometheus.MustRegister(durationCollectors...)
//
func InstrumentAWS(handlers *request.Handlers, buckets []float64) []prometheus.Collector {
	if buckets == nil {
		buckets = prometheus.DefBuckets
	}
	duration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "aws_api_request_duration_seconds",
			Help:    "Duration of AWS API requests.",
			Buckets: buckets,
		},
		[]string{LabelMethod, "service", "operation", LabelStatusCode},
	)
	instrumentAWSDuration(handlers, duration)
	return []prometheus.Collector{duration}
}

func instrumentAWSDuration(handlers *request.Handlers, o prometheus.ObserverVec) {
	handlers.Send.PushFrontNamed(request.NamedHandler{
		Name: "prometheus_metrics",
		Fn: func(r *request.Request) {
			timer := NewVecTimer(o)
			r.Handlers.CompleteAttempt.PushFront(func(r *request.Request) {
				labels := prometheus.Labels{
					LabelMethod:     r.Operation.HTTPMethod,
					"service":       r.ClientInfo.ServiceName,
					"operation":     r.Operation.Name,
					LabelStatusCode: strconv.Itoa(r.HTTPResponse.StatusCode),
				}
				timer.ObserveWith(labels)
			})
		},
	})
}
