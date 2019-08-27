package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"

	// "crypto/tls"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestInstrumentHTTP(t *testing.T) {
	r := prometheus.NewRegistry()

	srv := httptest.NewTLSServer(http.NotFoundHandler())
	defer srv.Close()

	baseClient := srv.Client()

	// Use a domain name instead of IP to test the DNS lookup duration metric.
	u := strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)
	baseClient.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify = true

	ca, err := InstrumentHTTP(baseClient, r, "", map[string]string{"recipient": "a"})
	if !assert.NoError(t, err) {
		return
	}

	cb, err := InstrumentHTTP(baseClient, r, "", map[string]string{"recipient": "b"})
	if !assert.NoError(t, err) {
		return
	}

	for i := 0; i < 15; i++ {
		_, err := ca.Head(u)
		assert.NoError(t, err)

		_, err = ca.Get(u)
		assert.NoError(t, err)

		_, err = ca.Post(u, "text/plain", nil)
		assert.NoError(t, err)

		_, err = cb.Head(u)
		assert.NoError(t, err)

		_, err = cb.Get(u)
		assert.NoError(t, err)

		_, err = cb.Post(u, "text/plain", nil)
		assert.NoError(t, err)
	}

	assertMetrics(t, r, 16)
}
