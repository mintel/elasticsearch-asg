package health

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	elastic "github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestCheckReadyRollingUpgrade_passing(t *testing.T) {
	client, _, mux, teardown := setup(t)
	defer teardown()
	disableCheckReadyRollingUpgrade = false
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
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(body)
		if err != nil {
			panic(err)
		}
	})
	err := CheckReadyRollingUpgrade(context.TODO(), client, zap.L().Named("rolling-upgrade"))
	assert.NoError(t, err)
}

func TestCheckReadyRollingUpgrade_timeout(t *testing.T) {
	client, _, mux, teardown := setup(t)
	defer teardown()
	disableCheckReadyRollingUpgrade = false
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
		time.Sleep(2 * timeout)
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(body)
		if err != nil {
			panic(err)
		}
	})
	err := CheckReadyRollingUpgrade(context.TODO(), client, zap.L().Named("rolling-upgrade"))
	assert.Error(t, err)
}

func TestCheckReadyRollingUpgrade_error(t *testing.T) {
	client, _, mux, teardown := setup(t)
	defer teardown()
	disableCheckReadyRollingUpgrade = false
	mux.HandleFunc("/_cat/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	err := CheckReadyRollingUpgrade(context.TODO(), client, zap.L().Named("rolling-upgrade"))
	assert.Error(t, err)
}

func TestCheckReadyRollingUpgrade_relo(t *testing.T) {
	client, _, mux, teardown := setup(t)
	defer teardown()
	disableCheckReadyRollingUpgrade = false
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
	err := CheckReadyRollingUpgrade(context.TODO(), client, zap.L().Named("rolling-upgrade"))
	assert.Error(t, err)
}

func TestCheckReadyRollingUpgrade_init(t *testing.T) {
	client, _, mux, teardown := setup(t)
	defer teardown()
	disableCheckReadyRollingUpgrade = false
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
	err := CheckReadyRollingUpgrade(context.TODO(), client, zap.L().Named("rolling-upgrade"))
	assert.Error(t, err)
}

func TestCheckReadyRollingUpgrade_red(t *testing.T) {
	client, _, mux, teardown := setup(t)
	defer teardown()
	disableCheckReadyRollingUpgrade = false
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
	err := CheckReadyRollingUpgrade(context.TODO(), client, zap.L().Named("rolling-upgrade"))
	assert.Error(t, err)
}

func TestCheckReadyRollingUpgrade_noop(t *testing.T) {
	client, _, mux, teardown := setup(t)
	defer teardown()
	disableCheckReadyRollingUpgrade = true
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
	err := CheckReadyRollingUpgrade(context.TODO(), client, zap.L().Named("rolling-upgrade"))
	assert.NoError(t, err)
}
