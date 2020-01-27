package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus" // Prometheus metrics.
	"github.com/stretchr/testify/assert"             // Test assertions e.g. equality.

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/defaults"
)

func TestInstrumentAWSDuration(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	r := prometheus.NewRegistry()

	config := defaults.Config()
	config.Region = "mock-region"
	config.EndpointResolver = aws.ResolveWithEndpoint(aws.Endpoint{
		URL:           server.URL,
		SigningRegion: config.Region,
	})

	err := InstrumentAWS(&config.Handlers, r, "", nil)
	if !assert.NoError(t, err) {
		return
	}

	svc1 := aws.NewClient(
		config,
		aws.Metadata{
			ServiceName:   "MockService1",
			SigningRegion: config.Region,
			APIVersion:    "2015-12-08",
			JSONVersion:   "1.1",
			TargetPrefix:  "MockServer",
		},
	)

	svc2 := aws.NewClient(
		config,
		aws.Metadata{
			ServiceName:   "MockService2",
			SigningRegion: config.Region,
			APIVersion:    "2015-12-08",
			JSONVersion:   "1.1",
			TargetPrefix:  "MockServer",
		},
	)

	for i := 0; i < 15; i++ {
		req := svc1.NewRequest(
			&aws.Operation{
				Name:       "DoThing",
				HTTPMethod: "GET",
			},
			nil,
			nil,
		)
		_ = req.Send()

		req = svc1.NewRequest(
			&aws.Operation{
				Name:       "DoOtherThing",
				HTTPMethod: "GET",
			},
			nil,
			nil,
		)
		_ = req.Send()

		req = svc2.NewRequest(
			&aws.Operation{
				Name:       "DoThing",
				HTTPMethod: "GET",
			},
			nil,
			nil,
		)
		_ = req.Send()

		req = svc2.NewRequest(
			&aws.Operation{
				Name:       "DoOtherThing",
				HTTPMethod: "GET",
			},
			nil,
			nil,
		)
		_ = req.Send()
	}

	assertMetrics(t, r, 2*2*2)
}
