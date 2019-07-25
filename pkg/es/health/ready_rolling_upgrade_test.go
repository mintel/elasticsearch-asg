package health

import (
	"net/http"
	"testing"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client
	"github.com/stretchr/testify/assert"    // Test assertion e.g. equality
	gock "gopkg.in/h2non/gock.v1"           // HTTP endpoint mocking

	"github.com/mintel/elasticsearch-asg/pkg/es" // Elasticsearch client extensions
)

func TestCheckReadyRollingUpgrade_passing(t *testing.T) {
	u, teardown := setup(t)
	defer teardown()
	defer gock.Off()
	// gock.Observe(gock.DumpRequest) // Log HTTP requests during test.
	check := CheckReadyRollingUpgrade(u)

	const (
		thisNode = "ce35b16d58ef"
		shard1   = "abc123"
		shard2   = "456xyz"
	)

	// The first run of the check should call the nodes info endpoint to get the node name.
	// There should be no other requests to the nodes info endpoint after this.
	// Then the /_cat/shards endpoint should be called, which will return shard1 on
	// thisNode which is INITIALIZING. This will cause the check to fail.
	gock.New(u).
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
	gock.New(u).
		Get("/_cat/shards").
		Reply(http.StatusOK).
		JSON(es.CatShardsResponse{
			es.CatShardsResponseRow{
				ID:               shard1,
				State:            "INITIALIZING",
				Node:             thisNode,
				PrimaryOrReplica: "p",
			},
		})
	err := check()
	assert.Error(t, err)
	assert.True(t, gock.IsDone())

	// This run of the check should call only the /_cat/shards endpoint.
	// It will return the shard1 (which has STARTED) and shard2 that is RELOCATING.
	// The RELOCATING shard will cause the check to fail.
	gock.New(u).
		Get("/_cat/shards").
		Reply(http.StatusOK).
		JSON(es.CatShardsResponse{
			es.CatShardsResponseRow{
				ID:    shard1,
				State: "STARTED",
				Node:  thisNode,
			},
			es.CatShardsResponseRow{
				ID:    shard2,
				State: "RELOCATING",
				// Node unspecified because it shouldn't matter which nodes are involved for a RELOCATING shard.
			},
		})
	err = check()
	assert.Error(t, err)
	assert.True(t, gock.IsDone())

	// This run of the check will pass because shard1 is start.
	// Shard2 will be ignored even though it's INITIALIZING because
	// it wasn't in the list of shards from the first run of the check.
	gock.New(u).
		Get("/_cat/shards").
		Reply(http.StatusOK).
		JSON(es.CatShardsResponse{
			es.CatShardsResponseRow{
				ID:    shard1,
				State: "STARTED",
				Node:  thisNode,
			},
			es.CatShardsResponseRow{
				ID:    shard2,
				State: "INITIALIZING",
				Node:  thisNode,
			},
		})
	err = check()
	assert.NoError(t, err)
	assert.True(t, gock.IsDone())

	// Additional check runs should always pass.
	gock.New(u).
		Get("/_cat/shards").
		Reply(http.StatusOK).
		JSON(es.CatShardsResponse{
			es.CatShardsResponseRow{
				ID:    shard1,
				State: "INITIALIZING",
				Node:  thisNode,
			},
			es.CatShardsResponseRow{
				ID:    shard2,
				State: "RELOCATING",
				// Node unspecified because it shouldn't matter which nodes are involved for a RELOCATING shard.
			},
		})
	err = check()
	assert.NoError(t, err)
	assert.True(t, gock.IsPending()) // The endpoint won't even be called.
}

func TestCheckReadyRollingUpgrade_error(t *testing.T) {
	u, teardown := setup(t)
	defer teardown()
	defer gock.Off()
	// gock.Observe(gock.DumpRequest) // Log HTTP requests during test.
	check := CheckReadyRollingUpgrade(u)
	gock.New(u).
		Get("/_nodes/_local/info").
		Reply(http.StatusInternalServerError).
		BodyString(http.StatusText(http.StatusInternalServerError))
	err := check()
	assert.Error(t, err)
	assert.True(t, gock.IsDone())
}
