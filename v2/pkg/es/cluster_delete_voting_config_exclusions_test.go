package es

import (
	"net/http"
	"testing"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client.
	"github.com/stretchr/testify/assert"    // Test assertions e.g. equality.
	gock "gopkg.in/h2non/gock.v1"           // HTTP request mocking.

	"github.com/mintel/elasticsearch-asg/v2/internal/pkg/testutil" // Testing utilities.
)

func TestClusterDeleteVotingConfigExclusion(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctx, _, teardown := testutil.ClientTestSetup(t)
		defer teardown()
		defer gock.CleanUnmatchedRequest()
		client, err := elastic.NewSimpleClient()
		if err != nil {
			panic(err)
		}

		gock.New(elastic.DefaultURL).
			Delete("/_cluster/voting_config_exclusions").
			Reply(http.StatusOK)

		_, err = NewClusterDeleteVotingConfigExclusion(client).Do(ctx)
		assert.NoError(t, err)
		assert.Condition(t, gock.IsDone)
	})

	t.Run("wait-true", func(t *testing.T) {
		ctx, _, teardown := testutil.ClientTestSetup(t)
		defer teardown()
		defer gock.CleanUnmatchedRequest()
		client, err := elastic.NewSimpleClient()
		if err != nil {
			panic(err)
		}

		gock.New(elastic.DefaultURL).
			MatchParam("wait_for_removal", "true").
			Delete("/_cluster/voting_config_exclusions").
			Reply(http.StatusOK)

		_, err = NewClusterDeleteVotingConfigExclusion(client).Wait(true).Do(ctx)
		assert.NoError(t, err)
		assert.Condition(t, gock.IsDone)
	})

	t.Run("wait-false", func(t *testing.T) {
		ctx, _, teardown := testutil.ClientTestSetup(t)
		defer teardown()
		defer gock.CleanUnmatchedRequest()
		client, err := elastic.NewSimpleClient()
		if err != nil {
			panic(err)
		}

		gock.New(elastic.DefaultURL).
			MatchParam("wait_for_removal", "false").
			Delete("/_cluster/voting_config_exclusions").
			Reply(http.StatusOK)

		_, err = NewClusterDeleteVotingConfigExclusion(client).Wait(false).Do(ctx)
		assert.NoError(t, err)
		assert.Condition(t, gock.IsDone)
	})

	t.Run("error", func(t *testing.T) {
		ctx, _, teardown := testutil.ClientTestSetup(t)
		defer teardown()
		defer gock.CleanUnmatchedRequest()
		client, err := elastic.NewSimpleClient()
		if err != nil {
			panic(err)
		}

		gock.New(elastic.DefaultURL).
			Delete("/_cluster/voting_config_exclusions").
			Reply(http.StatusInternalServerError).
			BodyString(http.StatusText(http.StatusInternalServerError))

		_, err = NewClusterDeleteVotingConfigExclusion(client).Do(ctx)
		assert.Error(t, err)
		assert.Condition(t, gock.IsDone)
	})
}
