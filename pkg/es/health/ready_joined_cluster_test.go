package health

import (
	"net/http"
	"testing"

	elastic "github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"
)

func TestCheckReadyJoinedCluster_passing(t *testing.T) {
	check, _, mux, teardown := setup(t, CheckReadyJoinedCluster)
	defer teardown()
	mux.On("GET", "/_cluster/state/_all/_all", nil, nil).Once().Return(http.StatusOK, nil, &elastic.ClusterStateResponse{
		ClusterName: "elasticsearch",
		Version:     16,
		StateUUID:   "808c1e3f-7fb5-4c97-b662-0d6be95f2f54",
	})
	err := check()
	assert.NoError(t, err)
	mux.AssertExpectations(t)
}

func TestCheckReadyJoinedCluster_timeout(t *testing.T) {
	check, _, mux, teardown := setup(t, CheckReadyJoinedCluster)
	defer teardown()
	mux.On("GET", "/_cluster/state/_all/_all", nil, nil).Once().After(DefaultHTTPTimeout*2).Return(http.StatusOK, nil, &elastic.ClusterStateResponse{
		ClusterName: "elasticsearch",
		Version:     16,
		StateUUID:   "808c1e3f-7fb5-4c97-b662-0d6be95f2f54",
	})
	err := check()
	assert.Error(t, err)
	mux.AssertExpectations(t)
}

func TestCheckReadyJoinedCluster_error(t *testing.T) {
	check, _, mux, teardown := setup(t, CheckReadyJoinedCluster)
	defer teardown()
	mux.On("GET", "/_cluster/state/_all/_all", nil, nil).Once().Return(http.StatusInternalServerError, nil, nil)
	err := check()
	assert.Error(t, err)
	mux.AssertExpectations(t)
}

func TestCheckReadyJoinedCluster_not_joined(t *testing.T) {
	check, _, mux, teardown := setup(t, CheckReadyJoinedCluster)
	defer teardown()
	mux.On("GET", "/_cluster/state/_all/_all", nil, nil).Once().Return(http.StatusOK, nil, &elastic.ClusterStateResponse{
		Version:   -1,
		StateUUID: "_na_",
	})
	err := check()
	assert.Error(t, err)
	mux.AssertExpectations(t)
}
