package main

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	elastic "github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-sdk-go/service/cloudwatch"

	esasg "github.com/mintel/elasticsearch-asg"
	"github.com/mintel/elasticsearch-asg/mocks/mockhttp"
	"github.com/mintel/elasticsearch-asg/pkg/str"
)

const delta = 0.001

func TestMakeCloudwatchData(t *testing.T) {
	mux := &mockhttp.Mux{}
	mux.On("GET", "/_nodes/_all/_all", nil, nil).Return(http.StatusOK, nil, helperLoadBytes(t, "nodes_info.json"))
	mux.On("GET", "/_nodes/stats", nil, nil).Return(http.StatusOK, nil, helperLoadBytes(t, "nodes_stats.json"))
	mux.On("GET", "/_cluster/settings", nil, nil).Return(http.StatusOK, nil, helperLoadBytes(t, "cluster_settings.json"))
	mux.On("GET", "/_cat/shards", nil, nil).Return(http.StatusOK, nil, "[]")
	server := httptest.NewServer(mux)
	defer server.Close()

	client, err := elastic.NewSimpleClient(elastic.SetURL(server.URL))
	if !assert.NoError(t, err) {
		return
	}

	esQuery := esasg.NewElasticsearchQueryService(client)
	nodes, err := esQuery.Nodes(context.TODO())
	if !assert.NoError(t, err) {
		return
	}

	metrics := MakeCloudwatchData(nodes)

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
		},
		"coordinate": {
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

	mux.AssertExpectations(t)
}

func helperLoadBytes(t *testing.T, name string) []byte {
	path := filepath.Join("testdata", name) // relative path
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to load test data file %s: %s", name, err)
	}
	return bytes
}
