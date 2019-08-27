package metrics

import (
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/prometheus/client_golang/prometheus"
)

// InstrumentAWS adds Prometheus metrics to an AWS client or session.
//
// A Gauge is observed for in-flight requests with labels
// for AWS service, operation name, and HTTP method labels.
//
// A duration histogram is observed for each AWS API request with
// labels for AWS service, operation name, HTTP method,
// and returned HTTP status code.
//
// IF the AWS clients retry on error, each attempt counts as a separate sample.
//
// Example:
//
//   sess := session.Must(session.NewSession())
//   InstrumentAWS(&sess.Handlers, prometheus.DefaultRegisterer, "", nil)
//
func InstrumentAWS(h *aws.Handlers, reg prometheus.Registerer, namespace string, constLabels map[string]string) error {
	i := &awsInstrumentation{
		duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace:   namespace,
				Subsystem:   "aws",
				Name:        "request_duration_seconds",
				Help:        "A histogram of AWS API request latencies.",
				Buckets:     prometheus.DefBuckets, // TODO: Define better buckets (maybe).
				ConstLabels: constLabels,
			},
			[]string{LabelRegion, LabelService, LabelOperation, LabelMethod, LabelStatusCode},
		),
		inflight: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "aws",
			Name:        "in_flight_requests",
			Help:        "A gauge of in-flight AWS API requests.",
			ConstLabels: constLabels,
		}, []string{LabelRegion, LabelService, LabelOperation, LabelMethod}),
	}
	if err := reg.Register(i); err != nil {
		return err
	}
	h.Send.PushFrontNamed(aws.NamedHandler{
		Name: "prometheus-send-start",
		Fn:   i.handleSend,
	})
	h.Retry.PushFrontNamed(aws.NamedHandler{
		Name: "prometheus-retry",
		Fn:   i.handleRetry,
	})
	return nil
}

type awsInstrumentation struct {
	duration *prometheus.HistogramVec
	inflight *prometheus.GaugeVec
}

// Describe implements prometheus.Collector interface.
func (i *awsInstrumentation) Describe(c chan<- *prometheus.Desc) {
	i.duration.Describe(c)
	i.inflight.Describe(c)
}

// Collect implements prometheus.Collector interface.
func (i *awsInstrumentation) Collect(c chan<- prometheus.Metric) {
	i.duration.Collect(c)
	i.inflight.Collect(c)
}

func (i *awsInstrumentation) handleSend(r *aws.Request) {
	labels := prometheus.Labels{
		LabelRegion:    r.Config.Region,
		LabelMethod:    r.Operation.HTTPMethod,
		LabelService:   r.Metadata.ServiceName,
		LabelOperation: r.Operation.Name,
	}

	timer := NewVecTimer(i.duration)
	i.inflight.With(labels).Inc()

	r.Handlers.Send.PushBackNamed(aws.NamedHandler{
		Name: "prometheus-send-end",
		Fn: func(r *aws.Request) {
			i.inflight.With(labels).Dec()
			labels[LabelStatusCode] = strconv.Itoa(r.HTTPResponse.StatusCode)
			timer.ObserveWith(labels)
		},
	})
}

func (i *awsInstrumentation) handleRetry(r *aws.Request) {
	labels := prometheus.Labels{
		LabelRegion:    r.Config.Region,
		LabelMethod:    r.Operation.HTTPMethod,
		LabelService:   r.Metadata.ServiceName,
		LabelOperation: r.Operation.Name,
	}

	timer := NewVecTimer(i.duration)
	i.inflight.With(labels).Inc()

	r.Handlers.AfterRetry.PushFrontNamed(aws.NamedHandler{
		Name: "prometheus-after-retry",
		Fn: func(r *aws.Request) {
			i.inflight.With(labels).Dec()
			labels[LabelStatusCode] = strconv.Itoa(r.HTTPResponse.StatusCode)
			timer.ObserveWith(labels)
		},
	})
}
