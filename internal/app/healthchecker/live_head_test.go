package healthchecker

import (
	"net/http"
	"testing"

	elastic "github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert" // Test assertion e.g. equality
	gock "gopkg.in/h2non/gock.v1"        // HTTP endpoint mocking

	"github.com/mintel/elasticsearch-asg/internal/pkg/testutil"
)

func TestCheckLiveHEAD(t *testing.T) {
	t.Run("passing", func(t *testing.T) {
		_, _, teardown := testutil.ClientTestSetup(t)
		defer teardown()
		defer gock.CleanUnmatchedRequest()
		client, err := elastic.NewSimpleClient()
		if err != nil {
			panic(err)
		}

		check := CheckLiveHEAD(client)

		gock.New(elastic.DefaultURL).
			Head("/").
			Reply(http.StatusOK)

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

		check := CheckLiveHEAD(client)

		gock.New(elastic.DefaultURL).
			Head("/").
			Reply(http.StatusInternalServerError).
			BodyString(http.StatusText(http.StatusInternalServerError))

		err = check()
		assert.Error(t, err)
		assert.Condition(t, gock.IsDone)
	})
}
