package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus" // Prometheus metrics.
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	LabelEvent = "event"
)

// InstrumentHTTP returns a new HTTP client instrumented
// with Prometheus metrics.
//
// If the base Client is nil, http.DefaultClient is used.
//
// A Gauge is observed for in-flight requests with a
// label for HTTP method.
//
// Histograms of duration are observed for DNS lookup,
// TLS negotiation, and overall request duration,
// with labels for HTTP method and returned
// HTTP status code.
//
// Example:
//
//   client := InstrumentHTTP(nil, prometheus.DefaultRegisterer, "", nil)
//
func InstrumentHTTP(base *http.Client, reg prometheus.Registerer, namespace string, constLabels map[string]string) (*http.Client, error) {
	if base == nil {
		base = http.DefaultClient
	}

	i := &httpClientInstrumentation{
		duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace:   namespace,
				Subsystem:   "http",
				Name:        "request_duration_seconds",
				Help:        "A histogram of HTTP request latencies.",
				Buckets:     prometheus.DefBuckets, // TODO: Define better buckets (maybe).
				ConstLabels: constLabels,
			},
			[]string{LabelStatusCode, LabelMethod},
		),
		inflight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "http",
			Name:        "in_flight_requests",
			Help:        "A gauge of in-flight HTTP requests.",
			ConstLabels: constLabels,
		}),
		dnsDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace:   namespace,
				Subsystem:   "http",
				Name:        "dns_duration_seconds",
				Help:        "Trace dns latency histogram.",
				Buckets:     []float64{.005, .01, .025, .05},
				ConstLabels: constLabels,
			},
			[]string{LabelEvent},
		),
		tlsDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace:   namespace,
				Subsystem:   "http",
				Name:        "tls_duration_seconds",
				Help:        "Trace tls latency histogram.",
				Buckets:     []float64{.05, .1, .25, .5},
				ConstLabels: constLabels,
			},
			[]string{LabelEvent},
		),
	}

	trace := &promhttp.InstrumentTrace{
		DNSStart: func(t float64) {
			i.dnsDuration.With(prometheus.Labels{LabelEvent: "dns_start"})
		},
		DNSDone: func(t float64) {
			i.dnsDuration.With(prometheus.Labels{LabelEvent: "dns_done"})
		},
		TLSHandshakeStart: func(t float64) {
			i.tlsDuration.With(prometheus.Labels{LabelEvent: "tls_handshake_start"})
		},
		TLSHandshakeDone: func(t float64) {
			i.tlsDuration.With(prometheus.Labels{LabelEvent: "tls_handshake_done"})
		},
	}

	transport := base.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	transport = promhttp.InstrumentRoundTripperDuration(i.duration, transport)
	transport = promhttp.InstrumentRoundTripperInFlight(i.inflight, transport)
	transport = promhttp.InstrumentRoundTripperTrace(trace, transport)

	if err := reg.Register(i); err != nil {
		return nil, err
	}

	c := &http.Client{
		CheckRedirect: base.CheckRedirect,
		Jar:           base.Jar,
		Timeout:       base.Timeout,
		Transport:     transport,
	}
	return c, nil
}

type httpClientInstrumentation struct {
	duration    *prometheus.HistogramVec
	inflight    prometheus.Gauge
	dnsDuration *prometheus.HistogramVec
	tlsDuration *prometheus.HistogramVec
}

// Describe implements prometheus.Collector interface.
func (i *httpClientInstrumentation) Describe(c chan<- *prometheus.Desc) {
	i.duration.Describe(c)
	i.inflight.Describe(c)
	i.dnsDuration.Describe(c)
	i.tlsDuration.Describe(c)
}

// Collect implements prometheus.Collector interface.
func (i *httpClientInstrumentation) Collect(c chan<- prometheus.Metric) {
	i.duration.Collect(c)
	i.inflight.Collect(c)
	i.dnsDuration.Collect(c)
	i.tlsDuration.Collect(c)
}
