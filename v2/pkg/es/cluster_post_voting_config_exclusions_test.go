package es

import (
	"net/http"
	"testing"

	"github.com/olivere/elastic/v7"      // Elasticsearch client.
	"github.com/stretchr/testify/assert" // Test assertions e.g. equality.
	gock "gopkg.in/h2non/gock.v1"        // HTTP request mocking.

	"github.com/mintel/elasticsearch-asg/v2/internal/pkg/testutil" // Testing utilities.
)

func TestClusterPostVotingConfigExclusions(t *testing.T) {
	const (
		nodeName = "foobar"
	)

	t.Run("success", func(t *testing.T) {
		ctx, _, teardown := testutil.ClientTestSetup(t)
		defer teardown()
		defer gock.CleanUnmatchedRequest()
		client, err := elastic.NewSimpleClient()
		if err != nil {
			panic(err)
		}

		gock.New(elastic.DefaultURL).
			Post("/_cluster/voting_config_exclusions/" + nodeName).
			Reply(http.StatusAccepted)

		_, err = NewClusterPostVotingConfigExclusion(client).Node(nodeName).Do(ctx)
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
			Post("/_cluster/voting_config_exclusions/" + nodeName).
			Reply(http.StatusInternalServerError).
			BodyString(http.StatusText(http.StatusInternalServerError))

		_, err = NewClusterPostVotingConfigExclusion(client).Node(nodeName).Do(ctx)
		assert.Error(t, err)
		assert.Condition(t, gock.IsDone)
	})

}
