package metrics

//go:generate sh -c "mockery '-name=Observer.*' -dir=$(go list -f '{{.Dir}}' github.com/prometheus/client_golang/prometheus)"

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/client/metadata"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	metricmocks "github.com/mintel/elasticsearch-asg/pkg/metrics/mocks"
)

func TestInstrumentAWSDuration(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
		} else {
			const code = http.StatusMethodNotAllowed
			http.Error(w, http.StatusText(code), code)
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	duration := &metricmocks.ObserverVec{}

	sess := session.Must(session.NewSession(&aws.Config{
		DisableSSL: aws.Bool(true),
		Endpoint:   aws.String(server.URL),
		Region:     aws.String(endpoints.UsEast1RegionID),
	}))
	instrumentAWSDuration(&sess.Handlers, duration)

	// Create a mock AWS client.
	const serviceName = "Mock"
	c := sess.ClientConfig(serviceName)
	svc := client.New(
		*c.Config,
		metadata.ClientInfo{
			ServiceName:   serviceName,
			SigningRegion: c.SigningRegion,
			Endpoint:      c.Endpoint,
			APIVersion:    "2015-12-08",
			JSONVersion:   "1.1",
			TargetPrefix:  serviceName + "Server",
		},
		c.Handlers,
	)

	t.Run("success", func(t *testing.T) {
		labels := prometheus.Labels{
			LabelMethod:     "GET",
			"service":       serviceName,
			"operation":     "DoSuccessfulThing",
			LabelStatusCode: strconv.Itoa(http.StatusOK),
		}
		m := &metricmocks.Observer{}
		duration.On("With", labels).Return(m).Once()
		m.On("Observe", mock.AnythingOfType("float64")).Return().Once()

		req := svc.NewRequest(&request.Operation{
			Name:       "DoSuccessfulThing",
			HTTPMethod: "GET",
		}, nil, nil)
		assert.NoError(t, req.Send())
		duration.AssertExpectations(t)
		m.AssertExpectations(t)
	})

	t.Run("error", func(t *testing.T) {
		labels := prometheus.Labels{
			LabelMethod:     "POST",
			"service":       serviceName,
			"operation":     "DoErrorThing",
			LabelStatusCode: strconv.Itoa(http.StatusMethodNotAllowed),
		}
		m := &metricmocks.Observer{}
		duration.On("With", labels).Return(m).Once()
		m.On("Observe", mock.AnythingOfType("float64")).Return().Once()

		req := svc.NewRequest(&request.Operation{
			Name:       "DoErrorThing",
			HTTPMethod: "POST",
		}, nil, nil)
		assert.Error(t, req.Send())
		duration.AssertExpectations(t)
		m.AssertExpectations(t)
	})
}
