package throttler

import (
	"net/http"
	"testing"

	"github.com/mintel/elasticsearch-asg/pkg/es"
	elastic "github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"
	gock "gopkg.in/h2non/gock.v1"

	"github.com/mintel/elasticsearch-asg/internal/pkg/testutil"
)

func TestClusterStateGetter(t *testing.T) {
	t.Run("bad", func(t *testing.T) {
		_, _, teardown := testutil.ClientTestSetup(t)
		defer teardown()
		client, err := elastic.NewSimpleClient()
		if !assert.NoError(t, err) {
			return
		}
		csg := NewClusterStateGetter(client)

		gock.New(elastic.DefaultURL).
			Get("/_cluster/health").
			Reply(http.StatusOK).
			JSON(&elastic.ClusterHealthResponse{
				Status:           "red",
				RelocatingShards: 1,
			})

		gock.New(elastic.DefaultURL).
			Get("/_recovery").
			ParamPresent("active_only").
			ParamPresent("detailed").
			Reply(http.StatusOK).
			JSON(es.IndicesRecoveryResponse{"index1": es.IndicesRecoveryResponseIndex{
				Shards: []*es.IndicesRecoveryResponseShard{
					&es.IndicesRecoveryResponseShard{
						Type: "store",
					},
				},
			}})

		s, err := csg.Get()
		if assert.NoError(t, err) {
			assert.Equal(t, "red", s.Status)
			assert.True(t, s.RelocatingShards)
			assert.True(t, s.RecoveringFromStore)
		}
		assert.Condition(t, gock.IsDone)
	})

	t.Run("good", func(t *testing.T) {
		_, _, teardown := testutil.ClientTestSetup(t)
		defer teardown()
		client, err := elastic.NewSimpleClient()
		if !assert.NoError(t, err) {
			return
		}
		csg := NewClusterStateGetter(client)
		csg.lastState = &ClusterState{
			Status:              "red",
			RelocatingShards:    true,
			RecoveringFromStore: true,
		}

		gock.New(elastic.DefaultURL).
			Get("/_cluster/health").
			ParamPresent("wait_for_status").
			ParamPresent("wait_for_no_relocating_shards").
			Reply(http.StatusOK).
			JSON(&elastic.ClusterHealthResponse{
				Status:           "yellow",
				RelocatingShards: 0,
			})

		gock.New(elastic.DefaultURL).
			Get("/_recovery").
			ParamPresent("active_only").
			ParamPresent("detailed").
			Reply(http.StatusOK).
			JSON(es.IndicesRecoveryResponse{})

		s, err := csg.Get()
		if assert.NoError(t, err) {
			assert.Equal(t, "yellow", s.Status)
			assert.False(t, s.RelocatingShards)
			assert.False(t, s.RecoveringFromStore)
		}
		assert.Condition(t, gock.IsDone)
	})
}
