package es

import (
	"net/http"
	"testing"

	elastic "github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"
	gock "gopkg.in/h2non/gock.v1"

	"github.com/mintel/elasticsearch-asg/internal/pkg/testutil"
)

func TestClusterPutSettingsService(t *testing.T) {
	t.Run("put", func(t *testing.T) {
		ctx, _, teardown := testutil.ClientTestSetup(t)
		defer teardown()
		defer gock.CleanUnmatchedRequest()
		client, err := elastic.NewSimpleClient()
		if err != nil {
			panic(err)
		}

		gock.New(elastic.DefaultURL).
			Put("/_cluster/settings").
			JSON(b{
				"transient":  b{"cluster.routing.allocation.exclude._name": "foo"},
				"persistent": b{"cluster.routing.allocation.exclude._name": "bar"},
			}).
			Reply(http.StatusOK).
			JSON(b{
				"transient":  b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": "foo"}}}}},
				"persistent": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": "bar"}}}}},
			})

		resp, err := NewClusterPutSettingsService(client).
			Transient("cluster.routing.allocation.exclude._name", "foo").
			Persistent("cluster.routing.allocation.exclude._name", "bar").
			Do(ctx)
		if assert.NoError(t, err) {
			assert.Equal(t, "foo", resp.Transient.Get("cluster.routing.allocation.exclude._name").String())
			assert.Equal(t, "bar", resp.Persistent.Get("cluster.routing.allocation.exclude._name").String())
		}
		assert.Condition(t, gock.IsDone)
	})

	t.Run("remove", func(t *testing.T) {
		ctx, _, teardown := testutil.ClientTestSetup(t)
		defer teardown()
		defer gock.CleanUnmatchedRequest()
		client, err := elastic.NewSimpleClient()
		if err != nil {
			panic(err)
		}

		gock.New(elastic.DefaultURL).
			Put("/_cluster/settings").
			JSON(b{
				"transient":  b{"cluster.routing.allocation.exclude._name": nil},
				"persistent": b{"cluster.routing.allocation.exclude._name": nil},
			}).
			Reply(http.StatusOK).
			JSON(b{})

		resp, err := NewClusterPutSettingsService(client).
			Transient("cluster.routing.allocation.exclude._name", nil).
			Persistent("cluster.routing.allocation.exclude._name", nil).
			Do(ctx)
		if assert.NoError(t, err) {
			assert.Nil(t, resp.Transient.Get("cluster.routing.allocation.exclude._name").Value())
			assert.Nil(t, resp.Persistent.Get("cluster.routing.allocation.exclude._name").Value())
		}
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
			Put("/_cluster/settings").
			JSON(b{
				"transient":  b{"cluster.routing.allocation.exclude._name": "foo"},
				"persistent": b{"cluster.routing.allocation.exclude._name": nil},
			}).
			Reply(http.StatusInternalServerError).
			BodyString(http.StatusText(http.StatusInternalServerError))

		_, err = NewClusterPutSettingsService(client).
			Transient("cluster.routing.allocation.exclude._name", "foo").
			Persistent("cluster.routing.allocation.exclude._name", nil).
			Do(ctx)
		assert.Error(t, err)
		assert.Condition(t, gock.IsDone)
	})

}
