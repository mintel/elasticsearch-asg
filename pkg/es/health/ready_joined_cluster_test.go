package health

import (
	"net/http"
	"testing"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client
	"github.com/stretchr/testify/assert"    // Test assertion e.g. equality
	gock "gopkg.in/h2non/gock.v1"           // HTTP endpoint mocking

	"github.com/mintel/elasticsearch-asg/internal/pkg/testutil"
)

func TestCheckReadyJoinedCluster_passing(t *testing.T) {
	defer setTestTimeout()
	ctx, _, teardown := testutil.ClientTestSetup(t)
	defer teardown()
	const u = "http://127.0.0.1:9200"

	check := CheckReadyJoinedCluster(ctx, u)
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
	defer setTestTimeout()
	ctx, _, teardown := testutil.ClientTestSetup(t)
	defer teardown()
	const u = "http://127.0.0.1:9200"

	check := CheckReadyJoinedCluster(ctx, u)
	gock.New(u).
		Get("/_cluster/state/_all/_all").
		Reply(http.StatusInternalServerError).
		BodyString(http.StatusText(http.StatusInternalServerError))
	err := check()
	assert.Error(t, err)
	assert.True(t, gock.IsDone())
}

func TestCheckReadyJoinedCluster_not_joined(t *testing.T) {
	defer setTestTimeout()
	ctx, _, teardown := testutil.ClientTestSetup(t)
	defer teardown()
	const u = "http://127.0.0.1:9200"

	check := CheckReadyJoinedCluster(ctx, u)
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
