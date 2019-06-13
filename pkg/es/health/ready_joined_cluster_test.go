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

func TestCheckReadyJoinedCluster_passing(t *testing.T) {
	client, _, mux, teardown := setup(t)
	defer teardown()
	mux.HandleFunc("/_cluster/state/_all/_all", func(w http.ResponseWriter, r *http.Request) {
		body, err := json.Marshal(&elastic.ClusterStateResponse{
			ClusterName: "elasticsearch",
			Version:     16,
			StateUUID:   "808c1e3f-7fb5-4c97-b662-0d6be95f2f54",
		})
		if err != nil {
			panic(err)
		}
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(body)
		if err != nil {
			panic(err)
		}
	})
	err := CheckReadyJoinedCluster(context.TODO(), client, zap.L().Named("joined-cluster"))
	assert.NoError(t, err)
}

func TestCheckReadyJoinedCluster_timeout(t *testing.T) {
	client, _, mux, teardown := setup(t)
	defer teardown()
	mux.HandleFunc("/_cluster/state/_all/_all", func(w http.ResponseWriter, r *http.Request) {
		body, err := json.Marshal(&elastic.ClusterStateResponse{
			ClusterName: "elasticsearch",
			Version:     16,
			StateUUID:   "808c1e3f-7fb5-4c97-b662-0d6be95f2f54",
		})
		if err != nil {
			panic(err)
		}
		time.Sleep(timeout * 2)
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(body)
		if err != nil {
			panic(err)
		}
	})
	err := CheckReadyJoinedCluster(context.TODO(), client, zap.L().Named("joined-cluster"))
	assert.Error(t, err)
}

func TestCheckReadyJoinedCluster_error(t *testing.T) {
	client, _, mux, teardown := setup(t)
	defer teardown()
	mux.HandleFunc("/_cluster/state/_all/_all", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	err := CheckReadyJoinedCluster(context.TODO(), client, zap.L().Named("joined-cluster"))
	assert.Error(t, err)
}

func TestCheckReadyJoinedCluster_not_joined(t *testing.T) {
	client, _, mux, teardown := setup(t)
	defer teardown()
	mux.HandleFunc("/_cluster/state/_all/_all", func(w http.ResponseWriter, r *http.Request) {
		body, err := json.Marshal(&elastic.ClusterStateResponse{
			Version:   -1,
			StateUUID: "_na_",
		})
		if err != nil {
			panic(err)
		}
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(body)
		if err != nil {
			panic(err)
		}
	})
	err := CheckReadyJoinedCluster(context.TODO(), client, zap.L().Named("joined-cluster"))
	assert.Error(t, err)
}
