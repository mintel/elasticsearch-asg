package es

import (
	"net/http"
	"testing"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client.
	"github.com/stretchr/testify/assert"    // Test assertions e.g. equality.
	gock "gopkg.in/h2non/gock.v1"           // HTTP request mocking.

	"github.com/mintel/elasticsearch-asg/v2/internal/pkg/testutil" // Testing utilities.
)

func TestClusterGetSettingsService(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctx, _, teardown := testutil.ClientTestSetup(t)
		defer teardown()
		defer gock.CleanUnmatchedRequest()
		client, err := elastic.NewSimpleClient()
		if err != nil {
			panic(err)
		}

		gock.New(elastic.DefaultURL).
			Get("/_cluster/settings").
			MatchParam("include_defaults", "true").
			Reply(http.StatusOK).
			BodyString(testutil.LoadTestData("cluster_settings_defaults.json"))

		resp, err := NewClusterGetSettingsService(client).Defaults(true).Do(ctx)
		assert.NoError(t, err)
		assert.NotEmpty(t, resp.Defaults.Map())
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
			Get("/_cluster/settings").
			Reply(http.StatusInternalServerError).
			BodyString(http.StatusText(http.StatusInternalServerError))

		_, err = NewClusterGetSettingsService(client).Do(ctx)
		assert.Error(t, err)
		assert.Condition(t, gock.IsDone)
	})
}
