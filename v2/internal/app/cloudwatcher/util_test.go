package cloudwatcher

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert" // Test assertions e.g. equality.
	"github.com/tidwall/gjson"           // Dynamic JSON parsing.

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/defaults"

	"github.com/mintel/elasticsearch-asg/v2/internal/pkg/testutil" // Testing utilities.
	"github.com/mintel/elasticsearch-asg/v2/pkg/es"                // Extensions to the Elasticsearch client.
)

func Test_compressPayload(t *testing.T) {
	const body = "this is the message body"

	req := makeRequest()
	req.SetBufferBody([]byte(body))
	if err := req.Build(); !assert.NoError(t, err) {
		return
	}

	compressPayload(req)

	cE := req.HTTPRequest.Header.Get("Content-Encoding")
	assert.Equal(t, "gzip", cE)

	b, err := ioutil.ReadAll(req.HTTPRequest.Body)
	if !assert.NoError(t, err) {
		return
	}

	g, err := gzip.NewReader(bytes.NewReader(b))
	if !assert.NoError(t, err) {
		return
	}
	defer g.Close()

	b, err = ioutil.ReadAll(g)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, []byte(body), b)

	_, err = g.Read(b)
	assert.Equal(t, io.EOF, err)
}

func makeRequest() *aws.Request {
	req := aws.New(
		aws.Config{
			Credentials:      aws.AnonymousCredentials,
			Region:           "us-east-1",
			EndpointResolver: aws.ResolveWithEndpointURL("https://mock-service.mock-region.amazonaws.com"),
		},
		aws.Metadata{
			ServiceName:   "Mock",
			APIVersion:    "2015-12-08",
			JSONVersion:   "1.1",
			TargetPrefix:  "MockServer",
			Endpoint:      "https://mock-service.mock-region.amazonaws.com",
			SigningRegion: "us-east-1",
		},
		defaults.Handlers(),
		nil,
		&aws.Operation{
			Name:       "APIName",
			HTTPMethod: "POST",
			HTTPPath:   "/",
		},
		struct{}{}, nil,
	)
	req.Handlers.UnmarshalMeta.Clear()
	req.Handlers.ValidateResponse.Clear()
	req.Handlers.Unmarshal.Clear()
	req.Handlers.Validate.Clear()
	req.Handlers.Send.Clear()
	return req
}

// nodeStatsTestData returns a nodeStatsSlice equal to the JSON
// in `testdata/nodes_stats.json`.
func nodeStatsTestData() NodeStatsSlice {
	return NodeStatsSlice{
		&NodeStats{
			Name:                   "i-001b1abab63133912",
			Roles:                  []string{"ingest"},
			ExcludedFromAllocation: false,
			VCPUs:                  _nvcpu,
			Load1m:                 0.03,
			Load5m:                 0.13,
			Load15m:                0.11,
			JVMHeapStats: JVMHeapStats{
				MaxBytes:  1960509440,
				UsedBytes: 159933184,
			},
			JVMHeapPools: map[string]JVMHeapStats{
				"young": JVMHeapStats{
					MaxBytes:  69795840,
					UsedBytes: 46170728,
				},
				"survivor": JVMHeapStats{
					MaxBytes:  8716288,
					UsedBytes: 38392,
				},
				"old": JVMHeapStats{
					MaxBytes:  1881997312,
					UsedBytes: 113761984,
				},
			},
			FilesystemTotalBytes:     5843333120,
			FilesystemAvailableBytes: 5498716160,
		},

		&NodeStats{
			Name:                   "i-0498ae3c83d833659",
			Roles:                  []string{"master"},
			ExcludedFromAllocation: false,
			VCPUs:                  _nvcpu,
			Load1m:                 0.02,
			Load5m:                 0.08,
			Load15m:                0.08,
			JVMHeapStats: JVMHeapStats{
				MaxBytes:  8157593600,
				UsedBytes: 133928896,
			},
			JVMHeapPools: map[string]JVMHeapStats{
				"young": JVMHeapStats{
					MaxBytes:  69795840,
					UsedBytes: 18089560,
				},
				"survivor": JVMHeapStats{
					MaxBytes:  8716288,
					UsedBytes: 28008,
				},
				"old": JVMHeapStats{
					MaxBytes:  8079081472,
					UsedBytes: 115843016,
				},
			},
			FilesystemTotalBytes:     68685922304,
			FilesystemAvailableBytes: 68583505920,
		},

		&NodeStats{
			Name:                   "i-05d5063ba7e93296c",
			Roles:                  []string{"master"},
			ExcludedFromAllocation: false,
			VCPUs:                  _nvcpu,
			Load1m:                 0.05,
			Load5m:                 0.08,
			Load15m:                0.07,
			JVMHeapStats: JVMHeapStats{
				MaxBytes:  8157593600,
				UsedBytes: 173551240,
			},
			JVMHeapPools: map[string]JVMHeapStats{
				"young": JVMHeapStats{
					MaxBytes:  69795840,
					UsedBytes: 51744120,
				},
				"survivor": JVMHeapStats{
					MaxBytes:  8716288,
					UsedBytes: 465616,
				},
				"old": JVMHeapStats{
					MaxBytes:  8079081472,
					UsedBytes: 121358600,
				},
			},
			FilesystemTotalBytes:     68685922304,
			FilesystemAvailableBytes: 68583505920,
		},

		&NodeStats{
			Name:                   "i-0aab86111990f2d0c",
			Roles:                  []string{"ingest"},
			ExcludedFromAllocation: false,
			VCPUs:                  _nvcpu,
			Load1m:                 0.05,
			Load5m:                 0.07,
			Load15m:                0.07,
			JVMHeapStats: JVMHeapStats{
				MaxBytes:  1960509440,
				UsedBytes: 179838944,
			},
			JVMHeapPools: map[string]JVMHeapStats{
				"young": JVMHeapStats{
					MaxBytes:  69795840,
					UsedBytes: 65873896,
				},
				"survivor": JVMHeapStats{
					MaxBytes:  8716288,
					UsedBytes: 52904,
				},
				"old": JVMHeapStats{
					MaxBytes:  1881997312,
					UsedBytes: 113912144,
				},
			},
			FilesystemTotalBytes:     5843333120,
			FilesystemAvailableBytes: 5498720256,
		},

		&NodeStats{
			Name:                   "i-0adf68017a253c05d",
			Roles:                  []string{"data"},
			ExcludedFromAllocation: true,
			VCPUs:                  _nvcpu,
			Load1m:                 0.2,
			Load5m:                 0.27,
			Load15m:                0.18,
			JVMHeapStats: JVMHeapStats{
				MaxBytes:  8004501504,
				UsedBytes: 5807773576,
			},
			JVMHeapPools: map[string]JVMHeapStats{
				"young": JVMHeapStats{
					MaxBytes:  69795840,
					UsedBytes: 3086392,
				},
				"survivor": JVMHeapStats{
					MaxBytes:  8716288,
					UsedBytes: 119760,
				},
				"old": JVMHeapStats{
					MaxBytes:  7925989376,
					UsedBytes: 5804567424,
				},
			},
			FilesystemTotalBytes:     474768064512,
			FilesystemAvailableBytes: 473605922816,
		},

		&NodeStats{
			Name:                   "i-0d681a8eb9510112d",
			Roles:                  []string{"ingest"},
			ExcludedFromAllocation: false,
			VCPUs:                  _nvcpu,
			Load1m:                 0,
			Load5m:                 0.04,
			Load15m:                0.07,
			JVMHeapStats: JVMHeapStats{
				MaxBytes:  1960509440,
				UsedBytes: 121222112,
			},
			JVMHeapPools: map[string]JVMHeapStats{
				"young": JVMHeapStats{
					MaxBytes:  69795840,
					UsedBytes: 6732640,
				},
				"survivor": JVMHeapStats{
					MaxBytes:  8716288,
					UsedBytes: 43560,
				},
				"old": JVMHeapStats{
					MaxBytes:  1881997312,
					UsedBytes: 114476448,
				},
			},
			FilesystemTotalBytes:     5843333120,
			FilesystemAvailableBytes: 5498695680,
		},

		&NodeStats{
			Name:                   "i-0ea13932cc8493d2b",
			Roles:                  []string{"data"},
			ExcludedFromAllocation: false,
			VCPUs:                  _nvcpu,
			Load1m:                 0.01,
			Load5m:                 0.09,
			Load15m:                0.13,
			JVMHeapStats: JVMHeapStats{
				MaxBytes:  8004501504,
				UsedBytes: 2662946720,
			},
			JVMHeapPools: map[string]JVMHeapStats{
				"young": JVMHeapStats{
					MaxBytes:  69795840,
					UsedBytes: 26404464,
				},
				"survivor": JVMHeapStats{
					MaxBytes:  8716288,
					UsedBytes: 4184832,
				},
				"old": JVMHeapStats{
					MaxBytes:  7925989376,
					UsedBytes: 2632368448,
				},
			},
			FilesystemTotalBytes:     474768064512,
			FilesystemAvailableBytes: 473649733632,
		},

		&NodeStats{
			Name:                   "i-0f0ea93320f56e140",
			Roles:                  []string{"master"},
			ExcludedFromAllocation: false,
			VCPUs:                  _nvcpu,
			Load1m:                 0.29,
			Load5m:                 0.41,
			Load15m:                0.31,
			JVMHeapStats: JVMHeapStats{
				MaxBytes:  8157593600,
				UsedBytes: 145529704,
			},
			JVMHeapPools: map[string]JVMHeapStats{
				"young": JVMHeapStats{
					MaxBytes:  69795840,
					UsedBytes: 30264952,
				},
				"survivor": JVMHeapStats{
					MaxBytes:  8716288,
					UsedBytes: 125816,
				},
				"old": JVMHeapStats{
					MaxBytes:  8079081472,
					UsedBytes: 115170624,
				},
			},
			FilesystemTotalBytes:     68685922304,
			FilesystemAvailableBytes: 68583505920,
		},

		&NodeStats{
			Name:                   "i-0f5c6d4d61d41b9fc",
			Roles:                  []string{"data"},
			ExcludedFromAllocation: false,
			VCPUs:                  _nvcpu,
			Load1m:                 0.1,
			Load5m:                 0.13,
			Load15m:                0.1,
			JVMHeapStats: JVMHeapStats{
				MaxBytes:  8004501504,
				UsedBytes: 5892614368,
			},
			JVMHeapPools: map[string]JVMHeapStats{
				"young": JVMHeapStats{
					MaxBytes:  69795840,
					UsedBytes: 2170264,
				},
				"survivor": JVMHeapStats{
					MaxBytes:  8716288,
					UsedBytes: 2469448,
				},
				"old": JVMHeapStats{
					MaxBytes:  7925989376,
					UsedBytes: 5887988040,
				},
			},
			FilesystemTotalBytes:     474768064512,
			FilesystemAvailableBytes: 474104512512,
		},
	}
}

func loadSettings() *es.ClusterGetSettingsResponse {
	data := testutil.LoadTestData("cluster_settings.json")
	result := gjson.ParseBytes([]byte(data))
	persistent := result.Get("persistent")
	transient := result.Get("transient")
	return &es.ClusterGetSettingsResponse{
		Persistent: &persistent,
		Transient:  &transient,
	}
}
