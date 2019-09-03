package cloudwatcher

//go:generate sh -c "mockery -name=EC2API -dir=$(go list -f '{{.Dir}}' github.com/aws/aws-sdk-go/service/ec2/ec2iface)"

import (
	"context"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"testing"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client
	"github.com/stretchr/testify/assert"    // Test assertions
	"go.uber.org/zap/zaptest"               // Logging for tests
	gock "gopkg.in/h2non/gock.v1"           // Mock HTTP endpoints

	// AWS clients and stuff
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/mintel/elasticsearch-asg/internal/app/cloudwatcher/mocks" // Mocked AWS client(s)
	"github.com/mintel/elasticsearch-asg/internal/pkg/elasticsearch"      // Complex Elasticsearch services
	"github.com/mintel/elasticsearch-asg/pkg/ctxlog"                      // Logger from context
	"github.com/mintel/elasticsearch-asg/pkg/str"                         // String utilities
)

const (
	delta     = 0.001
	localhost = "http://127.0.0.1:9200"
)

func TestMakeCloudwatchData(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := ctxlog.WithLogger(context.Background(), logger)

	defer gock.Off()
	gock.New(localhost).Get("/_nodes/_all/_all").
		Reply(http.StatusOK).Type("json").BodyString(loadTestData(t, "nodes_info.json"))
	gock.New(localhost).Get("/_nodes/stats").
		Reply(http.StatusOK).Type("json").BodyString(loadTestData(t, "nodes_stats.json"))
	gock.New(localhost).Get("/_cluster/settings").
		Reply(http.StatusOK).Type("json").BodyString(loadTestData(t, "cluster_settings.json"))
	gock.New(localhost).Get("/_cat/shards").
		Reply(http.StatusOK).Type("json").BodyString("[]")

	esClient, err := elastic.NewSimpleClient(elastic.SetURL(localhost))
	if !assert.NoError(t, err) {
		return
	}

	esQuery := elasticsearch.NewQuery(esClient)
	nodes, err := esQuery.Nodes(context.TODO())
	if !assert.NoError(t, err) {
		return
	}

	cpuOpts := &ec2.CpuOptions{
		CoreCount:      aws.Int64(1),
		ThreadsPerCore: aws.Int64(2),
	}
	instanceIDs := []string{
		"i-001b1abab63133912",
		"i-0adf68017a253c05d",
		"i-0f5c6d4d61d41b9fc",
		"i-0d681a8eb9510112d",
		"i-0f0ea93320f56e140",
		"i-0aab86111990f2d0c",
		"i-0ea13932cc8493d2b",
		"i-0498ae3c83d833659",
		"i-05d5063ba7e93296c",
	}
	instanceIDPtrs := make([]*string, len(instanceIDs))
	instances := make([]*ec2.Instance, 0)
	for i, id := range instanceIDs {
		instances = append(instances, &ec2.Instance{
			InstanceId: aws.String(id),
			CpuOptions: cpuOpts,
		})
		instanceIDPtrs[i] = aws.String(id)
	}

	ec2Svc := &mocks.EC2API{}
	ec2Svc.On("DescribeInstancesWithContext", ctx, &ec2.DescribeInstancesInput{
		InstanceIds: instanceIDPtrs,
	}).Once().Return(&ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{
			&ec2.Reservation{
				Instances: instances,
			},
		},
	}, error(nil))

	vcpuCounts, err := GetInstanceVCPUCount(ctx, ec2Svc, instanceIDs)
	if !assert.NoError(t, err) {
		return
	}

	metrics := MakeCloudwatchData(nodes, vcpuCounts)

	expected := map[string]map[string]float64{
		"all": {
			"CountExcludedFromAllocation": 1,
			"CountNodes":                  9,
			"FSAvailableBytes":            1421360168960,
			"FSMaxBytes":                  1424304193536,
			"FSUtilization":               0.20669914400035516,
			"GCOldCount":                  32,
			"GCOldTime":                   1.298,
			"GCYoungCount":                160678,
			"GCYoungTime":                 7145.693,
			"JVMOldPoolMax":               53661204480,
			"JVMOldPoolPeakMax":           53661204480,
			"JVMOldPoolPeakUsed":          18472704728,
			"JVMOldPoolUsed":              15019446728,
			"JVMOldPoolUtilization":       27.625622,
			"JVMSurvivorPoolMax":          78446592,
			"JVMSurvivorPoolPeakMax":      78446592,
			"JVMSurvivorPoolPeakUsed":     78446592,
			"JVMSurvivorPoolUsed":         7528336,
			"JVMSurvivorPoolUtilization":  0.013847,
			"JVMTotal":                    54367813632,
			"JVMUsed":                     15277338744,
			"JVMYoungPoolMax":             628162560,
			"JVMYoungPoolPeakMax":         628162560,
			"JVMYoungPoolPeakUsed":        628162560,
			"JVMYoungPoolUsed":            250537016,
			"JVMYoungPoolUtilization":     0.460819,
			"CountvCPU":                   18,
			"Load1m":                      0.75,
			"Load5m":                      1.3,
			"Load15m":                     1.12,
			"Load1mUtilization":           4.1667,
			"Load5mUtilization":           7.2222,
			"Load15mUtilization":          6.2222,
		},
		"master": {
			"CountExcludedFromAllocation": 0,
			"CountNodes":                  3,
			"GCOldCount":                  9,
			"GCOldTime":                   0.295,
			"GCYoungCount":                8854,
			"GCYoungTime":                 662.617000,
			"JVMOldPoolMax":               24237244416,
			"JVMOldPoolPeakMax":           24237244416,
			"JVMOldPoolPeakUsed":          352372240,
			"JVMOldPoolUsed":              352372240,
			"JVMOldPoolUtilization":       1.439854,
			"JVMSurvivorPoolMax":          26148864,
			"JVMSurvivorPoolPeakMax":      26148864,
			"JVMSurvivorPoolPeakUsed":     26148864,
			"JVMSurvivorPoolUsed":         619440,
			"JVMSurvivorPoolUtilization":  0.002531,
			"JVMTotal":                    24472780800,
			"JVMUsed":                     453009840,
			"JVMYoungPoolMax":             209387520,
			"JVMYoungPoolPeakMax":         209387520,
			"JVMYoungPoolPeakUsed":        209387520,
			"JVMYoungPoolUsed":            100098632,
			"JVMYoungPoolUtilization":     0.409020,
			"CountvCPU":                   6,
			"Load1m":                      0.36,
			"Load5m":                      0.57,
			"Load15m":                     0.46,
			"Load1mUtilization":           6.0,
			"Load5mUtilization":           9.5,
			"Load15mUtilization":          7.6667,
		},
		"data": {
			"CountExcludedFromAllocation": 1,
			"CountNodes":                  3,
			"FSAvailableBytes":            1421360168960,
			"FSMaxBytes":                  1424304193536,
			"FSUtilization":               0.20669914400035516,
			"GCOldCount":                  14,
			"GCOldTime":                   0.753000,
			"GCYoungCount":                147322,
			"GCYoungTime":                 6426.984,
			"JVMOldPoolMax":               23777968128,
			"JVMOldPoolPeakMax":           23777968128,
			"JVMOldPoolPeakUsed":          17778181912,
			"JVMOldPoolUsed":              14324923912,
			"JVMOldPoolUtilization":       59.653617,
			"JVMSurvivorPoolMax":          26148864,
			"JVMSurvivorPoolPeakMax":      26148864,
			"JVMSurvivorPoolPeakUsed":     26148864,
			"JVMSurvivorPoolUsed":         6774040,
			"JVMSurvivorPoolUtilization":  0.028209,
			"JVMTotal":                    24013504512,
			"JVMUsed":                     14363334664,
			"JVMYoungPoolMax":             209387520,
			"JVMYoungPoolPeakMax":         209387520,
			"JVMYoungPoolPeakUsed":        209387520,
			"JVMYoungPoolUsed":            31661120,
			"JVMYoungPoolUtilization":     0.131847,
			"CountvCPU":                   6,
			"Load1m":                      0.31,
			"Load5m":                      0.49,
			"Load15m":                     0.41,
			"Load1mUtilization":           5.1667,
			"Load5mUtilization":           8.1667,
			"Load15mUtilization":          6.8333,
		},
		"ingest": {
			"CountExcludedFromAllocation": 0,
			"CountNodes":                  3,
			"GCOldCount":                  9,
			"GCOldTime":                   0.25,
			"GCYoungCount":                4502,
			"GCYoungTime":                 56.092000,
			"JVMOldPoolMax":               5645991936,
			"JVMOldPoolPeakMax":           5645991936,
			"JVMOldPoolPeakUsed":          342150576,
			"JVMOldPoolUsed":              342150576,
			"JVMOldPoolUtilization":       5.817375,
			"JVMSurvivorPoolMax":          26148864,
			"JVMSurvivorPoolPeakMax":      26148864,
			"JVMSurvivorPoolPeakUsed":     26148864,
			"JVMSurvivorPoolUsed":         134856,
			"JVMSurvivorPoolUtilization":  0.002293,
			"JVMTotal":                    5881528320,
			"JVMUsed":                     460994240,
			"JVMYoungPoolMax":             209387520,
			"JVMYoungPoolPeakMax":         209387520,
			"JVMYoungPoolPeakUsed":        209387520,
			"JVMYoungPoolUsed":            118777264,
			"JVMYoungPoolUtilization":     2.019497,
			"CountvCPU":                   6,
			"Load1m":                      0.08,
			"Load5m":                      0.24,
			"Load15m":                     0.25,
			"Load1mUtilization":           1.3333,
			"Load5mUtilization":           4.0,
			"Load15mUtilization":          4.1667,
		},
	}

	getRole := func(metric *cloudwatch.MetricDatum) string {
		for _, d := range metric.Dimensions {
			if *d.Name == "Role" {
				return *d.Value
			}
		}
		panic("no role dimension")
	}

	for _, metric := range metrics {
		role := getRole(metric)
		metricName := *metric.MetricName
		metricValue := *metric.Value
		if str.In(metricName, "FSMaxBytes", "FSAvailableBytes", "FSUtilization") {
			assert.Contains(t, []string{"data", "all"}, role, "Only data nodes should have FS metrics")
		}
		if v, ok := expected[role][*metric.MetricName]; ok {
			assert.InDelta(t, v, metricValue, delta, "%s (%s) should = %v, but = %f", metricName, role, v, metricValue)
		}
	}

	assert.True(t, gock.IsDone())
}

// loadTestData is help to load test data from the `testdata` directory.
func loadTestData(t *testing.T, name string) string {
	path := filepath.Join("testdata", name) // relative path
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to load test data file %s: %s", name, err)
	}
	return string(bytes)
}
