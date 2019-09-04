package elasticsearch

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client
	"github.com/stretchr/testify/assert"    // Test assertions e.g. equality
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	gock "gopkg.in/h2non/gock.v1" // HTTP endpoint mocking

	"github.com/mintel/elasticsearch-asg/internal/pkg/testutil"
)

type QueryTestSuite struct {
	suite.Suite

	SUT *Query // System Under Test

	teardown func()
	logger   *zap.Logger
	ctx      context.Context
	uri      string
}

func TestQuery(t *testing.T) {
	suite.Run(t, new(QueryTestSuite))
}

func (suite *QueryTestSuite) SetupTest() {
	ctx, logger, teardown := testutil.ClientTestSetup(suite.T())
	suite.ctx = ctx
	suite.logger = logger
	suite.uri = "http://127.0.0.1:9200"
	esClient, err := elastic.NewSimpleClient(elastic.SetURL(suite.uri))
	if err != nil {
		panic(err)
	}
	suite.SUT = NewQuery(esClient)
	suite.teardown = func() {
		esClient.Stop()
		teardown()
	}
}

func (suite *QueryTestSuite) TearDownTest() {
	suite.teardown()
}

func (suite *QueryTestSuite) TestQuery_ClusterName() {
	const want = "mycluster"

	gock.New(suite.uri).
		Get("/_cluster/health").
		Reply(http.StatusOK).
		JSON(b{
			"active_primary_shards":            0,
			"active_shards":                    0,
			"active_shards_percent_as_number":  100.0,
			"cluster_name":                     want,
			"delayed_unassigned_shards":        0,
			"initializing_shards":              0,
			"number_of_data_nodes":             1,
			"number_of_in_flight_fetch":        0,
			"number_of_nodes":                  1,
			"number_of_pending_tasks":          0,
			"relocating_shards":                0,
			"status":                           "green",
			"task_max_waiting_in_queue_millis": 0,
			"timed_out":                        false,
			"unassigned_shards":                0,
		})
	got, err := suite.SUT.ClusterName(suite.ctx)
	suite.NoError(err)
	suite.Equal(want, got)
	suite.True(gock.IsDone())
}

func (suite *QueryTestSuite) TestQuery_ClusterHealth() {
	const clusterName = "mycluster"

	gock.New(suite.uri).
		Get("/_cluster/health").
		Reply(http.StatusOK).
		JSON(b{
			"active_primary_shards":            0,
			"active_shards":                    0,
			"active_shards_percent_as_number":  100.0,
			"cluster_name":                     clusterName,
			"delayed_unassigned_shards":        0,
			"initializing_shards":              0,
			"number_of_data_nodes":             1,
			"number_of_in_flight_fetch":        0,
			"number_of_nodes":                  1,
			"number_of_pending_tasks":          0,
			"relocating_shards":                0,
			"status":                           "green",
			"task_max_waiting_in_queue_millis": 0,
			"timed_out":                        false,
			"unassigned_shards":                0,
		})
	health, err := suite.SUT.ClusterHealth(suite.ctx)
	suite.NoError(err)
	suite.Equal(clusterName, health.ClusterName)
	suite.True(gock.IsDone())
}

func (suite *QueryTestSuite) TestQuery_Nodes() {
	gock.New(suite.uri).
		Get("/_nodes/stats").
		Reply(200).
		Type("json").
		BodyString(loadTestData("nodes_stats.json"))
	gock.New(suite.uri).
		Get("/_nodes/_all/_all").
		Reply(200).
		Type("json").
		BodyString(loadTestData("nodes_info.json"))
	gock.New(suite.uri).
		Get("/_cluster/settings").
		Reply(200).
		Type("json").
		BodyString(loadTestData("cluster_settings.json"))
	gock.New(suite.uri).
		Get("/_cat/shards").
		Reply(200).
		Type("json").
		BodyString(loadTestData("cat_shards.json"))

	nodes, err := suite.SUT.Nodes(suite.ctx)
	suite.NoError(err)
	suite.True(gock.IsDone())
	suite.Len(nodes, 9)
}

func (suite *QueryTestSuite) TestQuery_Node() {
	const nodeName = "i-0f5c6d4d61d41b9fc"
	gock.New(suite.uri).
		Get(fmt.Sprintf("/_nodes/%s/stats", nodeName)).
		Reply(http.StatusOK).
		Type("json").
		BodyString(loadTestData(fmt.Sprintf("nodes_stats_%s.json", nodeName)))
	gock.New(suite.uri).
		Get(fmt.Sprintf("/_nodes/%s/_all", nodeName)).
		Reply(http.StatusOK).
		Type("json").
		BodyString(loadTestData(fmt.Sprintf("nodes_info_%s.json", nodeName)))
	gock.New(suite.uri).
		Get("/_cluster/settings").
		Reply(http.StatusOK).
		Type("json").
		BodyString(loadTestData("cluster_settings.json"))
	gock.New(suite.uri).
		Get("/_cat/shards").
		Reply(http.StatusOK).
		Type("json").
		BodyString(loadTestData("cat_shards.json"))

	n, err := suite.SUT.Node(suite.ctx, nodeName)
	suite.NoError(err)
	suite.True(gock.IsDone())
	suite.NotNil(n)
	suite.Equal(nodeName, n.Name)
	suite.Equal([]string{"data"}, n.Roles)
	suite.Equal(map[string]string{
		"aws_availability_zone":  "us-east-2a",
		"aws_instance_type":      "i3.large",
		"aws_instance_lifecycle": "spot",
		"xpack.installed":        "true",
		"aws_instance_family":    "i3",
	}, n.Attributes)
	suite.Len(n.Shards, 1)
}

func (suite *QueryTestSuite) TestQuery_GetSnapshots() {
	const (
		repoName  = "myrepo"
		snapshot1 = "analytics-20190902t040001"
		snapshot2 = "analytics-20190903t040001"
	)

	suite.Run("all", func() {
		gock.New(suite.uri).
			Get(fmt.Sprintf("/_snapshot/%s/_all", repoName)).
			Reply(http.StatusOK).
			Type("json").
			BodyString(loadTestData("snapshots_get_all.json"))
		snapshots, err := suite.SUT.GetSnapshots(suite.ctx, repoName)
		suite.NoError(err)
		suite.True(gock.IsDone())
		suite.NotNil(snapshots)
	})

	suite.Run("some", func() {
		gock.New(suite.uri).
			Get(fmt.Sprintf("/_snapshot/%s/%s,%s", repoName, snapshot1, snapshot2)).
			Reply(http.StatusOK).
			Type("json").
			BodyString(loadTestData("snapshots_get_some.json"))
		snapshots, err := suite.SUT.GetSnapshots(suite.ctx, repoName, snapshot1, snapshot2)
		suite.NoError(err)
		suite.True(gock.IsDone())
		suite.Len(snapshots, 2)
	})
}

func TestParseShardNodes(t *testing.T) {
	testCases := []struct {
		desc  string
		input string
		want  []string
		err   bool
	}{
		{
			desc:  "unassigned-shard",
			input: "",
			want:  nil,
		},
		{
			desc:  "shard",
			input: "i-0968d7621b79cd73d",
			want:  []string{"i-0968d7621b79cd73d"},
		},
		{
			desc:  "relocating-shard",
			input: "i-0968d7621b79cd73d -> 10.2.4.58 kNe49LLvSqGXBn2s8Ffgyw i-0a2ed08df0e5cfff6",
			want:  []string{"i-0968d7621b79cd73d", "i-0a2ed08df0e5cfff6"},
		},
		{
			desc:  "error",
			input: "not a node",
			err:   true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			got, err := parseShardNodes(tC.input)
			assert.Equal(t, got, tC.want)
			if tC.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
