package healthchecker

import (
	"net/http"
	"testing"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client.
	"github.com/stretchr/testify/assert"    // Test assertions e.g. equality.
	gock "gopkg.in/h2non/gock.v1"           // HTTP request mocking.

	"github.com/mintel/elasticsearch-asg/internal/pkg/testutil" // Testing utilities.
)

func TestCheckReadyJoinedCluster(t *testing.T) {
	t.Run("passing", func(t *testing.T) {
		_, _, teardown := testutil.ClientTestSetup(t)
		defer teardown()
		defer gock.CleanUnmatchedRequest()
		client, err := elastic.NewSimpleClient()
		if err != nil {
			panic(err)
		}

		check := CheckReadyJoinedCluster(client)

		gock.New(elastic.DefaultURL).
			Get("/_cluster/state/_all/_all").
			Reply(http.StatusOK).
			JSON(&elastic.ClusterStateResponse{
				ClusterName: "elasticsearch",
				Version:     16,
				StateUUID:   "808c1e3f-7fb5-4c97-b662-0d6be95f2f54",
			})
		err = check()
		assert.NoError(t, err)
		assert.Condition(t, gock.IsDone)
	})

	t.Run("error", func(t *testing.T) {
		_, _, teardown := testutil.ClientTestSetup(t)
		defer teardown()
		defer gock.CleanUnmatchedRequest()
		client, err := elastic.NewSimpleClient()
		if err != nil {
			panic(err)
		}

		check := CheckReadyJoinedCluster(client)

		gock.New(elastic.DefaultURL).
			Get("/_cluster/state/_all/_all").
			Reply(http.StatusInternalServerError).
			BodyString(http.StatusText(http.StatusInternalServerError))
		err = check()
		assert.Error(t, err)
		assert.Condition(t, gock.IsDone)
	})

	t.Run("not-joined", func(t *testing.T) {
		_, _, teardown := testutil.ClientTestSetup(t)
		defer teardown()
		defer gock.CleanUnmatchedRequest()
		client, err := elastic.NewSimpleClient()
		if err != nil {
			panic(err)
		}

		check := CheckReadyJoinedCluster(client)

		gock.New(elastic.DefaultURL).
			Get("/_cluster/state/_all/_all").
			Reply(http.StatusOK).
			JSON(&elastic.ClusterStateResponse{
				Version:   -1,
				StateUUID: "_na_",
			})
		err = check()
		assert.Error(t, err)
		assert.Condition(t, gock.IsDone)
	})
}
