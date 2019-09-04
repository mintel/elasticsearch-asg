package elasticsearch

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	gock "gopkg.in/h2non/gock.v1" // HTTP endpoint mocking

	"github.com/mintel/elasticsearch-asg/pkg/ctxlog"
)

type CommandTestSuite struct {
	suite.Suite

	SUT *Command // System Under Test

	teardown func()
	ctx      context.Context
	uri      string
}

func TestCommand(t *testing.T) {
	suite.Run(t, new(CommandTestSuite))
}

func (suite *CommandTestSuite) SetupTest() {
	logger := zaptest.NewLogger(suite.T())
	teardownLogger1 := zap.ReplaceGlobals(logger)
	teardownLogger2 := zap.RedirectStdLog(logger)
	gock.Intercept() // Intercept HTTP requests sent via the default client.
	gock.Observe(gockObserver(logger))
	suite.uri = "http://127.0.0.1:9200"
	ctx, cancel := context.WithCancel(context.Background())
	suite.ctx = ctxlog.WithLogger(ctx, logger)
	esClient, err := elastic.NewSimpleClient(elastic.SetURL(suite.uri))
	if err != nil {
		panic(err)
	}
	suite.SUT = NewCommand(esClient)
	suite.teardown = func() {
		cancel()
		esClient.Stop()
		gock.OffAll()
		gock.Observe(nil)
		teardownLogger2()
		teardownLogger1()
		if err := logger.Sync(); err != nil {
			panic(err)
		}
	}
}

func (suite *CommandTestSuite) TearDownTest() {
	suite.teardown()
}

func (suite *CommandTestSuite) TestCommand_Drain() {
	const (
		node1Name          = "foo"
		node2Name          = "bar"
		sortedNodeNameList = "bar,foo"
	)

	// Drain a node.
	suite.Run("drain-first", func() {
		gock.New(suite.uri).
			Get("/_cluster/settings").
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{}})
		gock.New(suite.uri).
			Put("/_cluster/settings").
			JSON(b{"transient": b{"cluster.routing.allocation.exclude._name": node1Name}}).
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node1Name}}}}}})
		err := suite.SUT.Drain(suite.ctx, node1Name)
		suite.NoError(err)
		suite.True(gock.IsDone())
	})

	// Draining the same node again shouldn't need to PUT any settings.
	suite.Run("drain-first-again", func() {
		gock.New(suite.uri).
			Get("/_cluster/settings").
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node1Name}}}}}})
		err := suite.SUT.Drain(suite.ctx, node1Name)
		suite.NoError(err)
		suite.True(gock.IsDone())
	})

	// Draining a second node results in a comma-separated list in sorted order.
	suite.Run("drain-second", func() {
		gock.New(suite.uri).
			Get("/_cluster/settings").
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node1Name}}}}}})
		gock.New(suite.uri).
			Put("/_cluster/settings").
			JSON(b{"transient": b{"cluster.routing.allocation.exclude._name": sortedNodeNameList}}).
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": sortedNodeNameList}}}}}})
		err := suite.SUT.Drain(suite.ctx, node2Name)
		suite.NoError(err)
		suite.True(gock.IsDone())
	})
}

func (suite *CommandTestSuite) TestCommand_Undrain() {
	const (
		node1Name          = "foo"
		node2Name          = "bar"
		sortedNodeNameList = "bar,foo"
	)

	// Undrain a node.
	suite.Run("undrain-first", func() {
		gock.New(suite.uri).
			Get("/_cluster/settings").
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": sortedNodeNameList}}}}}})
		gock.New(suite.uri).
			Put("/_cluster/settings").
			JSON(b{"transient": b{"cluster.routing.allocation.exclude._name": node2Name}}).
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node2Name}}}}}})
		err := suite.SUT.Undrain(suite.ctx, node1Name)
		suite.NoError(err)
		suite.True(gock.IsDone())
	})

	// Undrain last node.
	suite.Run("undrain-last", func() {
		gock.New(suite.uri).
			Get("/_cluster/settings").
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node2Name}}}}}})
		gock.New(suite.uri).
			Put("/_cluster/settings").
			JSON(b{"transient": b{"cluster.routing.allocation.exclude._name": nil}}).
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{}})
		err := suite.SUT.Undrain(suite.ctx, node2Name)
		suite.NoError(err)
		suite.True(gock.IsDone())
	})
}

func (suite *CommandTestSuite) TestCommand_EnsureSnapshotRepo() {
	const (
		repoName = "myrepo"
		repoType = "s3"
	)
	repoSettings := map[string]string{
		"bucket": "foobar",
	}

	suite.Run("not-exist", func() {
		gock.New(suite.uri).
			Get(fmt.Sprintf("/_snapshot/%s", repoName)).
			Reply(http.StatusNotFound).
			JSON(b{
				"error": b{
					"reason": fmt.Sprintf("[%s] missing", repoName),
					"root_cause": []b{
						b{
							"reason": fmt.Sprintf("[%s] missing", repoName),
							"type":   "repository_missing_exception",
						},
					},
					"type": "repository_missing_exception",
				},
				"status": 404,
			})
		gock.New(suite.uri).
			Put(fmt.Sprintf("/_snapshot/%s", repoName)).
			JSON(b{"type": repoType, "settings": repoSettings}).
			Reply(http.StatusOK).
			JSON(b{"acknowledged": true})
		err := suite.SUT.EnsureSnapshotRepo(suite.ctx, repoName, repoType, repoSettings)
		suite.NoError(err)
		suite.True(gock.IsDone())
	})

	suite.Run("does-exist", func() {
		gock.New(suite.uri).
			Get(fmt.Sprintf("/_snapshot/%s", repoName)).
			Reply(http.StatusOK).
			JSON(b{
				repoName: b{
					"type":     repoType,
					"settings": repoSettings,
				},
			})
		err := suite.SUT.EnsureSnapshotRepo(suite.ctx, repoName, repoType, repoSettings)
		suite.NoError(err)
		suite.True(gock.IsDone())
	})

	suite.Run("wrong-type", func() {
		gock.New(suite.uri).
			Get(fmt.Sprintf("/_snapshot/%s", repoName)).
			Reply(http.StatusOK).
			JSON(b{
				repoName: b{
					"type":     "wrongtype",
					"settings": repoSettings,
				},
			})
		err := suite.SUT.EnsureSnapshotRepo(suite.ctx, repoName, repoType, repoSettings)
		suite.Error(err)
		suite.True(gock.IsDone())
	})
}

func (suite *CommandTestSuite) TestCommand_CreateSnapshot() {
	const (
		repoName     = "myrepo"
		format       = "analytics-20060102t150405"
		snapshotName = "analytics-20190903t040001"
	)
	now := time.Date(2019, 9, 3, 4, 0, 1, 0, time.UTC)

	gock.New(suite.uri).
		Put(fmt.Sprintf("/_snapshot/%s/%s", repoName, snapshotName)).
		MatchParam("wait_for_completion", "true").
		Reply(http.StatusOK).
		JSON(b{"acknowledged": true})
	err := suite.SUT.CreateSnapshot(suite.ctx, repoName, format, now)
	suite.NoError(err)
	suite.True(gock.IsDone())
}

func (suite *CommandTestSuite) TestCommand_DeleteSnapshot() {
	const (
		repoName     = "myrepo"
		snapshotName = "analytics-20190903t040001"
	)

	gock.New(suite.uri).
		Delete(fmt.Sprintf("/_snapshot/%s/%s", repoName, snapshotName)).
		Reply(http.StatusOK).
		JSON(b{"acknowledged": true})
	err := suite.SUT.DeleteSnapshot(suite.ctx, repoName, snapshotName)
	suite.NoError(err)
	suite.True(gock.IsDone())
}

func (suite *CommandTestSuite) TestCommand_ExcludeFromVoting() {
	const nodeName = "node1"

	gock.New(suite.uri).
		Post(fmt.Sprintf("/_cluster/voting_config_exclusions/%s", nodeName)).
		Reply(http.StatusOK).
		JSON(b{"acknowledged": true})
	err := suite.SUT.ExcludeFromVoting(suite.ctx, nodeName)
	suite.NoError(err)
	suite.True(gock.IsDone())
}

func (suite *CommandTestSuite) TestCommand_ClearVotingExclusions() {
	gock.New(suite.uri).
		Delete("_cluster/voting_config_exclusions").
		Reply(http.StatusOK).
		JSON(b{"acknowledged": true})
	err := suite.SUT.ClearVotingExclusions(suite.ctx)
	suite.NoError(err)
	suite.True(gock.IsDone())
}
