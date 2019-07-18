package health

import (
	"net/http"
	"testing"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client
	"github.com/stretchr/testify/assert"    // Test assertion e.g. equality
	gock "gopkg.in/h2non/gock.v1"           // HTTP endpoint mocking
)

func TestCheckReadyRollingUpgrade_passing(t *testing.T) {
	u, teardown := setup(t)
	defer teardown()
	defer gock.Off()
	check := CheckReadyRollingUpgrade(u)

	gock.New(u).
		Get("/_cat/health").
		Reply(http.StatusOK).
		JSON(&elastic.CatHealthResponse{
			elastic.CatHealthResponseRow{
				Epoch:               1557176440,
				Timestamp:           "21:00:40",
				Cluster:             "elasticsearch",
				Status:              "green",
				NodeTotal:           9,
				NodeData:            3,
				Shards:              2,
				Pri:                 1,
				Relo:                0,
				Init:                0,
				Unassign:            0,
				PendingTasks:        0,
				MaxTaskWaitTime:     "-",
				ActiveShardsPercent: "100%",
			},
		})
	err := check()
	assert.NoError(t, err)
	assert.True(t, gock.IsDone())

	// Calls after first passed check should always pass.
	gock.New(u).
		Get("/_cat/health").
		Reply(http.StatusOK).
		JSON(&elastic.CatHealthResponse{
			elastic.CatHealthResponseRow{
				Epoch:               1557176440,
				Timestamp:           "21:00:40",
				Cluster:             "elasticsearch",
				Status:              "yellow",
				NodeTotal:           9,
				NodeData:            3,
				Shards:              2,
				Pri:                 1,
				Relo:                0,
				Init:                0,
				Unassign:            0,
				PendingTasks:        0,
				MaxTaskWaitTime:     "-",
				ActiveShardsPercent: "100%",
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
	check := CheckReadyRollingUpgrade(u)
	gock.New(u).
		Get("/_cat/health").
		Reply(http.StatusInternalServerError).
		BodyString(http.StatusText(http.StatusInternalServerError))
	err := check()
	assert.Error(t, err)
	assert.True(t, gock.IsDone())
}

func TestCheckReadyRollingUpgrade_relo(t *testing.T) {
	u, teardown := setup(t)
	defer teardown()
	defer gock.Off()
	check := CheckReadyRollingUpgrade(u)

	gock.New(u).
		Get("/_cat/health").
		Reply(http.StatusOK).
		JSON(&elastic.CatHealthResponse{
			elastic.CatHealthResponseRow{
				Epoch:               1557176440,
				Timestamp:           "21:00:40",
				Cluster:             "elasticsearch",
				Status:              "yellow",
				NodeTotal:           9,
				NodeData:            3,
				Shards:              2,
				Pri:                 1,
				Relo:                1,
				Init:                0,
				Unassign:            0,
				PendingTasks:        0,
				MaxTaskWaitTime:     "-",
				ActiveShardsPercent: "100%",
			},
		})
	err := check()
	assert.Error(t, err)

	// Should error twice.
	err = check()
	assert.Error(t, err)

	assert.True(t, gock.IsDone())
}

func TestCheckReadyRollingUpgrade_init(t *testing.T) {
	u, teardown := setup(t)
	defer teardown()
	defer gock.Off()
	check := CheckReadyRollingUpgrade(u)

	gock.New(u).
		Get("/_cat/health").
		Reply(http.StatusOK).JSON(&elastic.CatHealthResponse{
		elastic.CatHealthResponseRow{
			Epoch:               1557176440,
			Timestamp:           "21:00:40",
			Cluster:             "elasticsearch",
			Status:              "yellow",
			NodeTotal:           9,
			NodeData:            3,
			Shards:              2,
			Pri:                 1,
			Relo:                0,
			Init:                1,
			Unassign:            0,
			PendingTasks:        0,
			MaxTaskWaitTime:     "-",
			ActiveShardsPercent: "100%",
		},
	})
	err := check()
	assert.Error(t, err)

	// Should error twice.
	err = check()
	assert.Error(t, err)

	assert.True(t, gock.IsDone())
}

func TestCheckReadyRollingUpgrade_red(t *testing.T) {
	u, teardown := setup(t)
	defer teardown()
	defer gock.Off()
	check := CheckReadyRollingUpgrade(u)

	gock.New(u).
		Get("/_cat/health").
		Reply(http.StatusOK).
		JSON(&elastic.CatHealthResponse{
			elastic.CatHealthResponseRow{
				Epoch:               1557176440,
				Timestamp:           "21:00:40",
				Cluster:             "elasticsearch",
				Status:              "red",
				NodeTotal:           9,
				NodeData:            3,
				Shards:              2,
				Pri:                 1,
				Relo:                0,
				Init:                0,
				Unassign:            0,
				PendingTasks:        0,
				MaxTaskWaitTime:     "-",
				ActiveShardsPercent: "100%",
			},
		})
	err := check()
	assert.Error(t, err)

	// Should error twice.
	err = check()
	assert.Error(t, err)

	assert.True(t, gock.IsDone())
}
