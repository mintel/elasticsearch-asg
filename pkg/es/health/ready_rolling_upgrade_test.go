package health

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	elastic "github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"
)

func TestCheckReadyRollingUpgrade_passing(t *testing.T) {
	check, _, mux, teardown := setup(t, CheckReadyRollingUpgrade)
	defer teardown()

	status := "green"

	mux.HandleFunc("/_cat/health", func(w http.ResponseWriter, r *http.Request) {
		resp := &elastic.CatHealthResponse{
			elastic.CatHealthResponseRow{
				Epoch:               1557176440,
				Timestamp:           "21:00:40",
				Cluster:             "elasticsearch",
				Status:              status,
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
		}
		body, err := json.Marshal(resp)
		if err != nil {
			panic(err)
		}
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(body)
		if err != nil {
			panic(err)
		}
	})
	err := check()
	assert.NoError(t, err)

	// Calls after first passed check should always pass.
	status = "yellow"
	err = check()
	assert.NoError(t, err)
}

func TestCheckReadyRollingUpgrade_timeout(t *testing.T) {
	check, _, mux, teardown := setup(t, CheckReadyRollingUpgrade)
	defer teardown()
	mux.HandleFunc("/_cat/health", func(w http.ResponseWriter, r *http.Request) {
		resp := &elastic.CatHealthResponse{
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
		}
		body, err := json.Marshal(resp)
		if err != nil {
			panic(err)
		}
		time.Sleep(2 * DefaultHTTPTimeout)
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(body)
		if err != nil {
			panic(err)
		}
	})
	err := check()
	assert.Error(t, err)

	// Should error twice.
	err = check()
	assert.Error(t, err)
}

func TestCheckReadyRollingUpgrade_error(t *testing.T) {
	check, _, mux, teardown := setup(t, CheckReadyRollingUpgrade)
	defer teardown()
	mux.HandleFunc("/_cat/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	err := check()
	assert.Error(t, err)
}

func TestCheckReadyRollingUpgrade_relo(t *testing.T) {
	check, _, mux, teardown := setup(t, CheckReadyRollingUpgrade)
	defer teardown()
	mux.HandleFunc("/_cat/health", func(w http.ResponseWriter, r *http.Request) {
		resp := &elastic.CatHealthResponse{
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
		}
		body, err := json.Marshal(resp)
		if err != nil {
			panic(err)
		}
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(body)
		if err != nil {
			panic(err)
		}
	})
	err := check()
	assert.Error(t, err)

	// Should error twice.
	err = check()
	assert.Error(t, err)
}

func TestCheckReadyRollingUpgrade_init(t *testing.T) {
	check, _, mux, teardown := setup(t, CheckReadyRollingUpgrade)
	defer teardown()
	mux.HandleFunc("/_cat/health", func(w http.ResponseWriter, r *http.Request) {
		resp := &elastic.CatHealthResponse{
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
		}
		body, err := json.Marshal(resp)
		if err != nil {
			panic(err)
		}
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(body)
		if err != nil {
			panic(err)
		}
	})
	err := check()
	assert.Error(t, err)

	// Should error twice.
	err = check()
	assert.Error(t, err)
}

func TestCheckReadyRollingUpgrade_red(t *testing.T) {
	check, _, mux, teardown := setup(t, CheckReadyRollingUpgrade)
	defer teardown()
	mux.HandleFunc("/_cat/health", func(w http.ResponseWriter, r *http.Request) {
		resp := &elastic.CatHealthResponse{
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
		}
		body, err := json.Marshal(resp)
		if err != nil {
			panic(err)
		}
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(body)
		if err != nil {
			panic(err)
		}
	})
	err := check()
	assert.Error(t, err)

	// Should error twice.
	err = check()
	assert.Error(t, err)
}
