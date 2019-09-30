package healthchecker

import (
	"net/http"
	"testing"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client.
	"github.com/stretchr/testify/assert"    // Test assertions e.g. equality.
	gock "gopkg.in/h2non/gock.v1"           // HTTP request mocking.

	"github.com/mintel/elasticsearch-asg/internal/pkg/testutil" // Testing utilities.
	"github.com/mintel/elasticsearch-asg/pkg/es"                // Extensions to the Elasticsearch client.
)

func TestCheckReadyRollingUpgrade(t *testing.T) {
	const (
		thisNode = "ce35b16d58ef"
		shard1   = "abc123"
		shard2   = "456xyz"
	)

	t.Run("passing", func(t *testing.T) {
		_, _, teardown := testutil.ClientTestSetup(t)
		defer teardown()
		defer gock.CleanUnmatchedRequest()
		client, err := elastic.NewSimpleClient()
		if err != nil {
			panic(err)
		}

		check := CheckReadyRollingUpgrade(client)

		// The first run of the check should call the nodes info endpoint to get the node name.
		// There should be no other requests to the nodes info endpoint after this.
		// Then the /_cat/shards endpoint should be called, which will return shard1 on
		// thisNode which is INITIALIZING. This will cause the check to fail.
		gock.New(elastic.DefaultURL).
			Get("/_nodes/_local/info").
			Reply(http.StatusOK).
			JSON(&elastic.NodesInfoResponse{
				ClusterName: "elasticsearch",
				Nodes: map[string]*elastic.NodesInfoNode{
					"5VX4RLpZQs2RHwJmM-dvbw": &elastic.NodesInfoNode{
						Name: thisNode,
					},
				},
			})
		gock.New(elastic.DefaultURL).
			Get("/_cat/shards").
			Reply(http.StatusOK).
			JSON(es.CatShardsResponse{
				es.CatShardsResponseRow{
					ID:               strPtr(shard1),
					State:            "INITIALIZING",
					Node:             strPtr(thisNode),
					PrimaryOrReplica: "p",
				},
			})
		err = check()
		assert.Error(t, err)
		assert.Condition(t, gock.IsDone)

		// This run of the check should call only the /_cat/shards endpoint.
		// It will return the shard1 (which has STARTED) and shard2 that is RELOCATING.
		// The RELOCATING shard will cause the check to fail.
		gock.New(elastic.DefaultURL).
			Get("/_cat/shards").
			Reply(http.StatusOK).
			JSON(es.CatShardsResponse{
				es.CatShardsResponseRow{
					ID:    strPtr(shard1),
					State: "STARTED",
					Node:  strPtr(thisNode),
				},
				es.CatShardsResponseRow{
					ID:    strPtr(shard2),
					State: "RELOCATING",
					// Node unspecified because it shouldn't matter which nodes are involved for a RELOCATING shard.
				},
			})
		err = check()
		assert.Error(t, err)
		assert.Condition(t, gock.IsDone)

		// This run of the check will pass because shard1 is start.
		// Shard2 will be ignored even though it's INITIALIZING because
		// it wasn't in the list of shards from the first run of the check.
		gock.New(elastic.DefaultURL).
			Get("/_cat/shards").
			Reply(http.StatusOK).
			JSON(es.CatShardsResponse{
				es.CatShardsResponseRow{
					ID:    strPtr(shard1),
					State: "STARTED",
					Node:  strPtr(thisNode),
				},
				es.CatShardsResponseRow{
					ID:    strPtr(shard2),
					State: "INITIALIZING",
					Node:  strPtr(thisNode),
				},
			})
		err = check()
		assert.NoError(t, err)
		assert.Condition(t, gock.IsDone)

		// Additional check runs should always pass.
		gock.New(elastic.DefaultURL).
			Get("/_cat/shards").
			Reply(http.StatusOK).
			JSON(es.CatShardsResponse{
				es.CatShardsResponseRow{
					ID:    strPtr(shard1),
					State: "INITIALIZING",
					Node:  strPtr(thisNode),
				},
				es.CatShardsResponseRow{
					ID:    strPtr(shard2),
					State: "RELOCATING",
					// Node unspecified because it shouldn't matter which nodes are involved for a RELOCATING shard.
				},
			})
		err = check()
		assert.NoError(t, err)
		assert.Condition(t, gock.IsPending) // The endpoint won't even be called.
	})

	t.Run("error", func(t *testing.T) {
		_, _, teardown := testutil.ClientTestSetup(t)
		defer teardown()
		defer gock.CleanUnmatchedRequest()
		client, err := elastic.NewSimpleClient()
		if err != nil {
			panic(err)
		}

		check := CheckReadyRollingUpgrade(client)

		gock.New(elastic.DefaultURL).
			Get("/_nodes/_local/info").
			Reply(http.StatusInternalServerError).
			BodyString(http.StatusText(http.StatusInternalServerError))

		err = check()
		assert.Error(t, err)
		assert.Condition(t, gock.IsDone)
	})

}
