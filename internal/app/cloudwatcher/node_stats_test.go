package cloudwatcher

import (
	"encoding/json"
	"testing"
	"time"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client.
	"github.com/stretchr/testify/suite"     // Test suite.

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"

	"github.com/mintel/elasticsearch-asg/internal/pkg/testutil" // Testing utilities.
	"github.com/mintel/elasticsearch-asg/pkg/es"                // Extensions to the Elasticsearch client.
)

const (
	nvcpu = 2
	delta = 0.001
)

type NodeStatsTestSuite struct {
	suite.Suite

	statsNodes          []*elastic.NodesStatsNode
	ec2Instances        []*EC2Instance
	transientExclusion  *es.ShardAllocationExcludeSettings
	persistentExclusion *es.ShardAllocationExcludeSettings
	wantStats           NodeStatsSlice
}

func TestNodeStats(t *testing.T) {
	suite.Run(t, &NodeStatsTestSuite{})
}

func (suite *NodeStatsTestSuite) SetupTest() {
	statsResp := &elastic.NodesStatsResponse{}
	err := json.Unmarshal([]byte(testutil.LoadTestData("nodes_stats.json")), statsResp)
	if err != nil {
		panic(err)
	}
	suite.statsNodes = statsRespNodes(statsResp)

	suite.ec2Instances = make([]*EC2Instance, 0)
	for _, n := range suite.statsNodes {
		suite.ec2Instances = append(suite.ec2Instances, &EC2Instance{
			ID:    n.Name,
			VCPUs: nvcpu,
		})
	}

	s := loadSettings()
	suite.transientExclusion = es.NewShardAllocationExcludeSettings(s.Transient)
	suite.persistentExclusion = es.NewShardAllocationExcludeSettings(s.Persistent)

	suite.wantStats = nodeStatsTestData()
}

func (suite *NodeStatsTestSuite) TestNewNodeStats() {
	got := make(NodeStatsSlice, len(suite.statsNodes))
	for i, n := range suite.statsNodes {
		s, err := NewNodeStats(
			n,
			suite.ec2Instances[i],
			suite.transientExclusion,
			suite.persistentExclusion,
		)
		suite.Require().NoError(err)
		got[i] = s
	}
	suite.Equal(suite.wantStats, got)
}

func (suite *NodeStatsTestSuite) TestNodeStats_HasRole() {
	tests := []struct {
		name string
		s    *NodeStats
		want map[string]bool
	}{
		{
			name: "multiple",
			s: &NodeStats{
				Roles: []string{"data", "ingest", "master"}, // Must be sorted.
			},
			want: map[string]bool{
				"all":        true,
				"master":     true,
				"data":       true,
				"ingest":     true,
				"ml":         false,
				"coordinate": false,
			},
		},
		{
			name: "none",
			s: &NodeStats{
				Roles: []string{}, // Must be sorted.
			},
			want: map[string]bool{
				"all":        true,
				"master":     false,
				"data":       false,
				"ingest":     false,
				"ml":         false,
				"coordinate": true,
			},
		},
	}
	for _, tt := range tests {
		suite.Run(tt.name, func() {
			for r, b := range tt.want {
				suite.Run(r, func() {
					got := tt.s.HasRole(r)
					suite.Equal(b, got)
				})
			}
		})
	}
}

func (suite *NodeStatsTestSuite) TestNodeStatsSlice_Aggregate() {
	dimensions := []cloudwatch.Dimension{
		cloudwatch.Dimension{
			Name:  aws.String("Cluster"),
			Value: aws.String("name"),
		},
	}
	got := suite.wantStats.Aggregate(dimensions)

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("CountNodes"),
		Unit:              cloudwatch.StandardUnitCount,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		Value:             aws.Float64(9),
	})

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("CountvCPU"),
		Unit:              cloudwatch.StandardUnitCount,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		Value:             aws.Float64(nvcpu * 9),
	})

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("Load1m"),
		Unit:              cloudwatch.StandardUnitNone,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		StatisticValues: &cloudwatch.StatisticSet{
			Maximum:     aws.Float64(0.29),
			Minimum:     aws.Float64(0),
			Sum:         aws.Float64(0.75),
			SampleCount: aws.Float64(9),
		},
	})

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("Load5m"),
		Unit:              cloudwatch.StandardUnitNone,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		StatisticValues: &cloudwatch.StatisticSet{
			Maximum:     aws.Float64(0.41),
			Minimum:     aws.Float64(0.04),
			Sum:         aws.Float64(1.3),
			SampleCount: aws.Float64(9),
		},
	})

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("Load15m"),
		Unit:              cloudwatch.StandardUnitNone,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		StatisticValues: &cloudwatch.StatisticSet{
			Maximum:     aws.Float64(0.31),
			Minimum:     aws.Float64(0.07),
			Sum:         aws.Float64(1.12),
			SampleCount: aws.Float64(9),
		},
	})

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("Load1mUtilization"),
		Unit:              cloudwatch.StandardUnitPercent,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		Value:             aws.Float64(4.1667),
	})

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("Load5mUtilization"),
		Unit:              cloudwatch.StandardUnitPercent,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		Value:             aws.Float64(7.2222),
	})

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("Load15mUtilization"),
		Unit:              cloudwatch.StandardUnitPercent,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		Value:             aws.Float64(6.2222),
	})

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("CountExcludedFromAllocation"),
		Unit:              cloudwatch.StandardUnitCount,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		Value:             aws.Float64(1),
	})

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("JVMMaxBytes"),
		Unit:              cloudwatch.StandardUnitBytes,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		StatisticValues: &cloudwatch.StatisticSet{
			Maximum:     aws.Float64(8157593600),
			Minimum:     aws.Float64(1960509440),
			Sum:         aws.Float64(54367813632),
			SampleCount: aws.Float64(9),
		},
	})

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("JVMUsedBytes"),
		Unit:              cloudwatch.StandardUnitBytes,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		StatisticValues: &cloudwatch.StatisticSet{
			Maximum:     aws.Float64(5892614368),
			Minimum:     aws.Float64(121222112),
			Sum:         aws.Float64(15277338744),
			SampleCount: aws.Float64(9),
		},
	})

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("JVMUtilization"),
		Unit:              cloudwatch.StandardUnitPercent,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		Value:             aws.Float64(28.1),
	})

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("FSTotalBytes"),
		Unit:              cloudwatch.StandardUnitBytes,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		StatisticValues: &cloudwatch.StatisticSet{
			Maximum:     aws.Float64(474768064512),
			Minimum:     aws.Float64(474768064512),
			Sum:         aws.Float64(1424304193536),
			SampleCount: aws.Float64(3),
		},
	})

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("FSAvailableBytes"),
		Unit:              cloudwatch.StandardUnitBytes,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		StatisticValues: &cloudwatch.StatisticSet{
			Maximum:     aws.Float64(474104512512),
			Minimum:     aws.Float64(473605922816),
			Sum:         aws.Float64(1421360168960),
			SampleCount: aws.Float64(3),
		},
	})

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("FSUtilization"),
		Unit:              cloudwatch.StandardUnitPercent,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		Value:             aws.Float64(0.20669914400035516),
	})

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("JVMYoungPoolMaxBytes"),
		Unit:              cloudwatch.StandardUnitBytes,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		StatisticValues: &cloudwatch.StatisticSet{
			Maximum:     aws.Float64(69795840),
			Minimum:     aws.Float64(69795840),
			Sum:         aws.Float64(628162560),
			SampleCount: aws.Float64(9),
		},
	})

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("JVMYoungPoolUsedBytes"),
		Unit:              cloudwatch.StandardUnitBytes,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		StatisticValues: &cloudwatch.StatisticSet{
			Maximum:     aws.Float64(65873896),
			Minimum:     aws.Float64(2170264),
			Sum:         aws.Float64(250537016),
			SampleCount: aws.Float64(9),
		},
	})

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("JVMYoungPoolUtilization"),
		Unit:              cloudwatch.StandardUnitPercent,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		Value:             aws.Float64(39.884105159021255),
	})

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("JVMSurvivorPoolMaxBytes"),
		Unit:              cloudwatch.StandardUnitBytes,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		StatisticValues: &cloudwatch.StatisticSet{
			Maximum:     aws.Float64(8716288),
			Minimum:     aws.Float64(8716288),
			Sum:         aws.Float64(78446592),
			SampleCount: aws.Float64(9),
		},
	})

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("JVMSurvivorPoolUsedBytes"),
		Unit:              cloudwatch.StandardUnitBytes,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		StatisticValues: &cloudwatch.StatisticSet{
			Maximum:     aws.Float64(4184832),
			Minimum:     aws.Float64(28008),
			Sum:         aws.Float64(7528336),
			SampleCount: aws.Float64(9),
		},
	})

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("JVMSurvivorPoolUtilization"),
		Unit:              cloudwatch.StandardUnitPercent,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		Value:             aws.Float64(9.59676616671888),
	})

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("JVMOldPoolMaxBytes"),
		Unit:              cloudwatch.StandardUnitBytes,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		StatisticValues: &cloudwatch.StatisticSet{
			Maximum:     aws.Float64(8079081472),
			Minimum:     aws.Float64(1881997312),
			Sum:         aws.Float64(53661204480),
			SampleCount: aws.Float64(9),
		},
	})

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("JVMOldPoolUsedBytes"),
		Unit:              cloudwatch.StandardUnitBytes,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		StatisticValues: &cloudwatch.StatisticSet{
			Maximum:     aws.Float64(5887988040),
			Minimum:     aws.Float64(113761984),
			Sum:         aws.Float64(15019446728),
			SampleCount: aws.Float64(9),
		},
	})

	suite.ContainsMetric(got, cloudwatch.MetricDatum{
		MetricName:        aws.String("JVMOldPoolUtilization"),
		Unit:              cloudwatch.StandardUnitPercent,
		Dimensions:        dimensions,
		StorageResolution: aws.Int64(1),
		Value:             aws.Float64(27.989395455329147),
	})
}

func (suite *NodeStatsTestSuite) ContainsMetric(s []cloudwatch.MetricDatum, v cloudwatch.MetricDatum) bool {
	name := aws.StringValue(v.MetricName)
	now := time.Now()
	found := false
	ok := true
	for _, m := range s {
		if aws.StringValue(m.MetricName) != name {
			continue
		}
		found = true
		ok = ok && suite.ElementsMatch(
			v.Dimensions, m.Dimensions,
			"metric %s doesn't have expected Dimensions", name,
		)

		ok = ok && suite.InDelta(
			aws.Float64Value(v.Value), aws.Float64Value(m.Value), delta,
			"metric %s doesn't have expected Value", name,
		)

		if suite.True((v.StatisticValues == nil) == (m.StatisticValues == nil)) && m.StatisticValues != nil {
			ok = ok && suite.InDelta(
				aws.Float64Value(v.StatisticValues.Maximum), aws.Float64Value(m.StatisticValues.Maximum), delta,
				"metric %s.StatisticValues.Maximum doesn't have the expected value", name,
			)

			ok = ok && suite.InDelta(
				aws.Float64Value(v.StatisticValues.Minimum), aws.Float64Value(m.StatisticValues.Minimum), delta,
				"metric %s.StatisticValues.Minimum doesn't have the expected value", name,
			)

			ok = ok && suite.InDelta(
				aws.Float64Value(v.StatisticValues.Sum), aws.Float64Value(m.StatisticValues.Sum), delta,
				"metric %s.StatisticValues.Sum doesn't have the expected value", name,
			)

			ok = ok && suite.InDelta(
				aws.Float64Value(v.StatisticValues.SampleCount), aws.Float64Value(m.StatisticValues.SampleCount), delta,
				"metric %s.StatisticValues.SampleCount doesn't have the expected value", name,
			)
		} else {
			ok = false
		}

		ok = ok && suite.Equal(
			aws.Int64Value(v.StorageResolution), aws.Int64Value(m.StorageResolution),
			"metric %s doesn't have expected StorageResolution", name,
		)

		ok = ok && suite.Nil(
			m.Values,
			"metric %s doesn't have expected Values", name,
		)

		ok = ok && suite.Nil(
			m.Counts,
			"metric %s doesn't have expected Counts", name,
		)

		ok = ok && suite.WithinDuration(
			now, aws.TimeValue(m.Timestamp), time.Second,
			"metric %s doesn't have expected Timestamp", name,
		)
		break
	}
	if !found {
		suite.T().Errorf("metric %s not found", name)
		return false
	}
	return ok
}
