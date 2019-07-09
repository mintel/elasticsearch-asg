package health

import (
	"net/http"
	"testing"

	elastic "github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"
)

func TestCheckReadyRollingUpgrade_passing(t *testing.T) {
	check, _, mux, teardown := setup(t, CheckReadyRollingUpgrade)
	defer teardown()

	mux.On("GET", "/_cat/health", nil, nil).Once().Return(http.StatusOK, nil, &elastic.CatHealthResponse{
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

	mux.On("GET", "/_cat/health", nil, nil).Return(http.StatusOK, nil, &elastic.CatHealthResponse{
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

	err := check()
	assert.NoError(t, err)

	// Calls after first passed check should always pass.
	err = check()
	assert.NoError(t, err)

	mux.AssertExpectations(t)
}

func TestCheckReadyRollingUpgrade_timeout(t *testing.T) {
	check, _, mux, teardown := setup(t, CheckReadyRollingUpgrade)
	defer teardown()

	mux.On("GET", "/_cat/health", nil, nil).After(2*DefaultHTTPTimeout).Return(http.StatusOK, nil, &elastic.CatHealthResponse{
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
	assert.Error(t, err)

	// Should error twice.
	err = check()
	assert.Error(t, err)

	mux.AssertExpectations(t)
}

func TestCheckReadyRollingUpgrade_error(t *testing.T) {
	check, _, mux, teardown := setup(t, CheckReadyRollingUpgrade)
	defer teardown()
	mux.On("GET", "/_cat/health", nil, nil).Return(http.StatusInternalServerError, nil, http.StatusText(http.StatusInternalServerError))
	err := check()
	assert.Error(t, err)
	mux.AssertExpectations(t)
}

func TestCheckReadyRollingUpgrade_relo(t *testing.T) {
	check, _, mux, teardown := setup(t, CheckReadyRollingUpgrade)
	defer teardown()

	mux.On("GET", "/_cat/health", nil, nil).Return(http.StatusOK, nil, &elastic.CatHealthResponse{
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

	mux.AssertExpectations(t)
}

func TestCheckReadyRollingUpgrade_init(t *testing.T) {
	check, _, mux, teardown := setup(t, CheckReadyRollingUpgrade)
	defer teardown()

	mux.On("GET", "/_cat/health", nil, nil).Return(http.StatusOK, nil, &elastic.CatHealthResponse{
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

	mux.AssertExpectations(t)
}

func TestCheckReadyRollingUpgrade_red(t *testing.T) {
	check, _, mux, teardown := setup(t, CheckReadyRollingUpgrade)
	defer teardown()

	mux.On("GET", "/_cat/health", nil, nil).Return(http.StatusOK, nil, &elastic.CatHealthResponse{
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

	mux.AssertExpectations(t)
}
