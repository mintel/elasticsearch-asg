package drainer

import (
	"context"
	"net/http"
	"testing"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client.
	"github.com/stretchr/testify/suite"     // Test suite.
	gock "gopkg.in/h2non/gock.v1"           // HTTP request mocking.

	"github.com/mintel/elasticsearch-asg/v2/internal/pkg/testutil" // Testing utilities.
	"github.com/mintel/elasticsearch-asg/v2/pkg/es"                // Extensions to the Elasticsearch client.
)

type ElasticsearchFacadeTestSuite struct {
	suite.Suite

	Ctx context.Context
	SUT *ElasticsearchFacade // System Under Test

	teardown func()
}

func TestElasticsearchFacade(t *testing.T) {
	suite.Run(t, &ElasticsearchFacadeTestSuite{})
}

func (suite *ElasticsearchFacadeTestSuite) SetupTest() {
	ctx, _, teardown := testutil.ClientTestSetup(suite.T())
	c, err := elastic.NewSimpleClient()
	if err != nil {
		panic(err)
	}
	suite.Ctx = ctx
	suite.SUT = NewElasticsearchFacade(c)
	suite.teardown = teardown
}

func (suite *ElasticsearchFacadeTestSuite) TeardownTest() {
	suite.teardown()
}

func (suite *ElasticsearchFacadeTestSuite) TestGetState() {
	suite.Run("success", func() {
		defer gock.CleanUnmatchedRequest()

		gock.New(elastic.DefaultURL).
			Get("/_cluster/settings").
			Reply(http.StatusOK).
			BodyString(testutil.LoadTestData("cluster_settings.json"))

		gock.New(elastic.DefaultURL).
			Get("/_nodes/_all/http").
			Reply(http.StatusOK).
			BodyString(testutil.LoadTestData("nodes_info.json"))

		gock.New(elastic.DefaultURL).
			Get("/_cat/shards").
			Reply(http.StatusOK).
			BodyString(testutil.LoadTestData("cat_shards.json"))

		got, err := suite.SUT.GetState(suite.Ctx)
		suite.NoError(err)
		want := &ClusterState{
			Nodes: []string{
				"i-001b1abab63133912",
				"i-0498ae3c83d833659",
				"i-05d5063ba7e93296c",
				"i-0aab86111990f2d0c",
				"i-0adf68017a253c05d",
				"i-0d681a8eb9510112d",
				"i-0ea13932cc8493d2b",
				"i-0f0ea93320f56e140",
				"i-0f5c6d4d61d41b9fc",
			},
			Shards: map[string]int{
				"i-0adf68017a253c05d": 1,
				"i-0f5c6d4d61d41b9fc": 1,
			},
			Exclusions: &es.ShardAllocationExcludeSettings{
				Name: []string{"i-0adf68017a253c05d"},
				Attr: make(map[string][]string),
			},
		}
		suite.Equal(want, got)
		suite.Condition(gock.IsDone)
	})

	suite.Run("error", func() {
		defer gock.CleanUnmatchedRequest()

		gock.New(elastic.DefaultURL).
			Get("/_cluster/settings").
			Reply(http.StatusOK).
			BodyString(testutil.LoadTestData("cluster_settings.json"))

		gock.New(elastic.DefaultURL).
			Get("/_nodes/_all/http").
			Reply(http.StatusOK).
			BodyString(testutil.LoadTestData("nodes_info.json"))

		gock.New(elastic.DefaultURL).
			Get("/_cat/shards").
			Reply(http.StatusInternalServerError).
			BodyString(http.StatusText(http.StatusInternalServerError))

		got, err := suite.SUT.GetState(suite.Ctx)
		suite.Error(err)
		suite.Nil(got)
		suite.Condition(gock.IsDone)
	})
}

func (suite *ElasticsearchFacadeTestSuite) TestDrainNodes() {
	const (
		node1Name          = "foo"
		node2Name          = "bar"
		sortedNodeNameList = "bar,foo"
	)

	// Drain a node.
	suite.Run("success", func() {
		defer gock.CleanUnmatchedRequest()

		gock.New(elastic.DefaultURL).
			Get("/_cluster/settings").
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{}})

		gock.New(elastic.DefaultURL).
			Put("/_cluster/settings").
			JSON(b{"transient": b{"cluster.routing.allocation.exclude._name": node1Name}}).
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node1Name}}}}}})

		err := suite.SUT.DrainNodes(suite.Ctx, []string{node1Name})
		suite.NoError(err)
		suite.Condition(gock.IsDone)
	})

	// Draining the same node again shouldn't need to PUT any settings.
	suite.Run("no-change", func() {
		defer gock.CleanUnmatchedRequest()

		gock.New(elastic.DefaultURL).
			Get("/_cluster/settings").
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node1Name}}}}}})

		err := suite.SUT.DrainNodes(suite.Ctx, []string{node1Name})
		suite.NoError(err)
		suite.Condition(gock.IsDone)
	})

	// Draining a second node results in a comma-separated list in sorted order.
	suite.Run("sorted", func() {
		defer gock.CleanUnmatchedRequest()

		gock.New(elastic.DefaultURL).
			Get("/_cluster/settings").
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node1Name}}}}}})

		gock.New(elastic.DefaultURL).
			Put("/_cluster/settings").
			JSON(b{"transient": b{"cluster.routing.allocation.exclude._name": sortedNodeNameList}}).
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": sortedNodeNameList}}}}}})

		err := suite.SUT.DrainNodes(suite.Ctx, []string{node2Name})
		suite.NoError(err)
		suite.Condition(gock.IsDone)
	})

	suite.Run("error", func() {
		defer gock.CleanUnmatchedRequest()

		err := suite.SUT.DrainNodes(suite.Ctx, []string{"foobar"})
		suite.Error(err)
		suite.Condition(gock.IsDone)
	})
}

func (suite *ElasticsearchFacadeTestSuite) TestUndrainNodes() {
	const (
		node1Name          = "foo"
		node2Name          = "bar"
		sortedNodeNameList = "bar,foo"
	)

	// Undrain a node.
	suite.Run("success", func() {
		defer gock.CleanUnmatchedRequest()

		gock.New(elastic.DefaultURL).
			Get("/_cluster/settings").
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": sortedNodeNameList}}}}}})

		gock.New(elastic.DefaultURL).
			Put("/_cluster/settings").
			JSON(b{"transient": b{"cluster.routing.allocation.exclude._name": node2Name}}).
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node2Name}}}}}})

		err := suite.SUT.UndrainNodes(suite.Ctx, []string{node1Name})
		suite.NoError(err)
		suite.Condition(gock.IsDone)
	})

	// Undrain last node.
	suite.Run("success2", func() {
		defer gock.CleanUnmatchedRequest()

		gock.New(elastic.DefaultURL).
			Get("/_cluster/settings").
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node2Name}}}}}})

		gock.New(elastic.DefaultURL).
			Put("/_cluster/settings").
			JSON(b{"transient": b{"cluster.routing.allocation.exclude._name": nil}}).
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{}})

		err := suite.SUT.UndrainNodes(suite.Ctx, []string{node2Name})
		suite.NoError(err)
		suite.Condition(gock.IsDone)
	})

	suite.Run("error", func() {
		defer gock.CleanUnmatchedRequest()

		gock.New(elastic.DefaultURL).
			Get("/_cluster/settings").
			Reply(http.StatusInternalServerError).
			BodyString(http.StatusText(http.StatusInternalServerError))

		err := suite.SUT.UndrainNodes(suite.Ctx, []string{node1Name})
		suite.Error(err)
		suite.Condition(gock.IsDone)
	})
}
