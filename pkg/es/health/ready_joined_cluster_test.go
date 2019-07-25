package health

import (
	"net/http"
	"testing"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client
	"github.com/stretchr/testify/assert"    // Test assertion e.g. equality
	gock "gopkg.in/h2non/gock.v1"           // HTTP endpoint mocking
)

func TestCheckReadyJoinedCluster_passing(t *testing.T) {
	u, teardown := setup(t)
	defer teardown()
	defer gock.Off()
	// gock.Observe(gock.DumpRequest) // Log HTTP requests during test.
	check := CheckReadyJoinedCluster(u)
	gock.New(u).
		Get("/_cluster/state/_all/_all").
		Reply(http.StatusOK).
		JSON(&elastic.ClusterStateResponse{
			ClusterName: "elasticsearch",
			Version:     16,
			StateUUID:   "808c1e3f-7fb5-4c97-b662-0d6be95f2f54",
		})
	err := check()
	assert.NoError(t, err)
	assert.True(t, gock.IsDone())
}

func TestCheckReadyJoinedCluster_error(t *testing.T) {
	u, teardown := setup(t)
	defer teardown()
	defer gock.Off()
	// gock.Observe(gock.DumpRequest) // Log HTTP requests during test.
	check := CheckReadyJoinedCluster(u)
	gock.New(u).
		Get("/_cluster/state/_all/_all").
		Reply(http.StatusInternalServerError).
		BodyString(http.StatusText(http.StatusInternalServerError))
	err := check()
	assert.Error(t, err)
	assert.True(t, gock.IsDone())
}

func TestCheckReadyJoinedCluster_not_joined(t *testing.T) {
	u, teardown := setup(t)
	defer teardown()
	defer gock.Off()
	// gock.Observe(gock.DumpRequest) // Log HTTP requests during test.
	check := CheckReadyJoinedCluster(u)
	gock.New(u).
		Get("/_cluster/state/_all/_all").
		Reply(http.StatusOK).
		JSON(&elastic.ClusterStateResponse{
			Version:   -1,
			StateUUID: "_na_",
		})
	err := check()
	assert.Error(t, err)
	assert.True(t, gock.IsDone())
}
