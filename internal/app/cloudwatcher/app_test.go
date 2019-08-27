package cloudwatcher

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/dgraph-io/ristretto"
	"github.com/mintel/elasticsearch-asg/internal/app/cloudwatcher/mocks"

	elastic "github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"
	gock "gopkg.in/h2non/gock.v1"

	"github.com/mintel/elasticsearch-asg/internal/pkg/testutil"
)

func TestApp_getClusterName(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		_, _, teardown := testutil.ClientTestSetup(t)
		teardown()
		c, err := elastic.NewSimpleClient()
		if err != nil {
			panic(err)
		}
		app := &App{}
		app.clients.Elasticsearch = c
		const cluster = "MyCluster"
		gock.New(elastic.DefaultURL).
			Get("/_cluster/health").
			Reply(http.StatusOK).
			JSON(&elastic.ClusterHealthResponse{ClusterName: cluster})
		got, err := app.getClusterName()
		assert.NoError(t, err)
		assert.Equal(t, cluster, got)
	})

	t.Run("error", func(t *testing.T) {
		_, _, teardown := testutil.ClientTestSetup(t)
		teardown()
		c, err := elastic.NewSimpleClient()
		if err != nil {
			panic(err)
		}
		app := &App{}
		app.clients.Elasticsearch = c
		gock.New(elastic.DefaultURL).
			Get("/_cluster/health").
			Reply(http.StatusInternalServerError).
			BodyString(http.StatusText(http.StatusInternalServerError))
		got, err := app.getClusterName()
		assert.Error(t, err)
		assert.Zero(t, got)
	})
}

func TestApp_getNodes(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		_, _, teardown := testutil.ClientTestSetup(t)
		teardown()
		c, err := elastic.NewSimpleClient()
		if err != nil {
			panic(err)
		}
		app := &App{}
		app.clients.Elasticsearch = c
		data := testutil.LoadTestData("nodes_stats.json")
		resp := &elastic.NodesStatsResponse{}
		if err := json.Unmarshal([]byte(data), resp); err != nil {
			panic(err)
		}
		want := statsRespNodes(resp)
		gock.New(elastic.DefaultURL).
			Get("/_nodes/stats/os,jvm,fs").
			Reply(http.StatusOK).
			JSON(resp)
		got, err := app.getNodes()
		assert.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("error", func(t *testing.T) {
		_, _, teardown := testutil.ClientTestSetup(t)
		teardown()
		c, err := elastic.NewSimpleClient()
		if err != nil {
			panic(err)
		}
		app := &App{}
		app.clients.Elasticsearch = c
		gock.New(elastic.DefaultURL).
			Get("/_nodes/stats/os,jvm,fs").
			Reply(http.StatusInternalServerError).
			BodyString(http.StatusText(http.StatusInternalServerError))
		got, err := app.getNodes()
		assert.Error(t, err)
		assert.Zero(t, got)
	})
}

func TestApp_getInstances(t *testing.T) {
	app := &App{}
	m := &mocks.EC2{}
	m.Test(t)
	app.clients.EC2 = m
	ids := []string{"i-123456789abc", "i-987654321def"}

	t.Run("success", func(t *testing.T) {
		m.Test(t)
		cache, err := ristretto.NewCache(&ristretto.Config{
			NumCounters: 1000 * 10,
			MaxCost:     1000,
			BufferItems: 64,
		})
		if err != nil {
			panic(err)
		}
		app.ec2Instances = cache

		input := &ec2.DescribeInstancesInput{InstanceIds: ids}
		output := &ec2.DescribeInstancesOutput{
			Reservations: []ec2.Reservation{
				ec2.Reservation{
					Instances: []ec2.Instance{
						ec2.Instance{
							InstanceId: aws.String("i-987654321def"),
							CpuOptions: &ec2.CpuOptions{
								CoreCount:      aws.Int64(4),
								ThreadsPerCore: aws.Int64(2),
							},
						},
						ec2.Instance{
							InstanceId: aws.String("i-123456789abc"),
							CpuOptions: &ec2.CpuOptions{
								CoreCount:      aws.Int64(4),
								ThreadsPerCore: aws.Int64(2),
							},
						},
					},
				},
			},
		}
		m.On("DescribeInstancesRequest", input).
			Return(output, error(nil)).
			Twice() // Twice because the paginator will also call DescribeInstancesRequest to copy the request object.

		want := []*EC2Instance{ // Should be sorted by ID.
			&EC2Instance{
				ID:    "i-123456789abc",
				VCPUs: 8,
			},
			&EC2Instance{
				ID:    "i-987654321def",
				VCPUs: 8,
			},
		}

		got, err := app.getInstances(ids)
		assert.NoError(t, err)
		assert.Equal(t, want, got)

		// Second call should hit the cache.
		time.Sleep(10 * time.Millisecond)
		got, err = app.getInstances(ids)
		assert.NoError(t, err)
		assert.Equal(t, want, got)

		m.AssertExpectations(t)
	})

	t.Run("error", func(t *testing.T) {
		m.Test(t)
		cache, err := ristretto.NewCache(&ristretto.Config{
			NumCounters: 1000 * 10,
			MaxCost:     1000,
			BufferItems: 64,
		})
		if err != nil {
			panic(err)
		}
		app.ec2Instances = cache

		input := &ec2.DescribeInstancesInput{InstanceIds: ids}
		output := (*ec2.DescribeInstancesOutput)(nil)
		m.On("DescribeInstancesRequest", input).
			Return(output, assert.AnError).
			Twice() // Twice because the paginator will also call DescribeInstancesRequest to copy the request object.

		got, err := app.getInstances(ids)
		assert.Equal(t, assert.AnError, err)
		assert.Zero(t, got)

		m.AssertExpectations(t)
	})
}

func TestApp_getSettings(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		_, _, teardown := testutil.ClientTestSetup(t)
		teardown()
		c, err := elastic.NewSimpleClient()
		if err != nil {
			panic(err)
		}
		app := &App{}
		app.clients.Elasticsearch = c
		gock.New(elastic.DefaultURL).
			Get("/_cluster/settings").
			Reply(http.StatusOK).
			BodyString(testutil.LoadTestData("cluster_settings.json"))
		got, err := app.getSettings()
		assert.NoError(t, err)
		assert.Equal(
			t,
			"i-0adf68017a253c05d",
			got.Transient.Get("cluster.routing.allocation.exclude._name").String(),
		)

	})

	t.Run("error", func(t *testing.T) {
		_, _, teardown := testutil.ClientTestSetup(t)
		teardown()
		c, err := elastic.NewSimpleClient()
		if err != nil {
			panic(err)
		}
		app := &App{}
		app.clients.Elasticsearch = c
		gock.New(elastic.DefaultURL).
			Get("/_cluster/settings").
			Reply(http.StatusInternalServerError).
			BodyString(http.StatusText(http.StatusInternalServerError))
		got, err := app.getSettings()
		assert.Error(t, err)
		assert.Zero(t, got)
	})
}

func TestApp_pushCloudwatchData(t *testing.T) {
	const (
		namespace  = "Foo"
		metricname = "Bar"
	)

	app := &App{
		flags: &Flags{
			Namespace: namespace,
		},
		inst: NewInstrumentation("", nil),
	}
	m := &mocks.CloudWatch{}
	m.Test(t)
	app.clients.CloudWatch = m
	datums := []cloudwatch.MetricDatum{
		cloudwatch.MetricDatum{
			MetricName:        aws.String(metricname),
			Unit:              cloudwatch.StandardUnitCount,
			StorageResolution: aws.Int64(1),
			Value:             aws.Float64(1),
		},
	}
	for i := 0; i < (2*batchSize)-1; i++ {
		datums = append(datums, datums[0])
	}

	t.Run("success", func(t *testing.T) {
		m.Test(t)

		input1 := &cloudwatch.PutMetricDataInput{
			Namespace:  aws.String(namespace),
			MetricData: datums[:batchSize],
		}
		input2 := &cloudwatch.PutMetricDataInput{
			Namespace:  aws.String(namespace),
			MetricData: datums[batchSize:],
		}
		output := &cloudwatch.PutMetricDataOutput{}

		m.On("PutMetricDataRequest", input1).Return(output, error(nil)).Once()
		m.On("PutMetricDataRequest", input2).Return(output, error(nil)).Once()

		err := app.pushCloudwatchData(datums)
		assert.NoError(t, err)

		m.AssertExpectations(t)
	})

	t.Run("error", func(t *testing.T) {
		m.Test(t)

	})
}
