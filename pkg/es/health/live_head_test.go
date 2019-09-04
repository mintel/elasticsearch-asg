package health

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert" // Test assertion e.g. equality
	gock "gopkg.in/h2non/gock.v1"        // HTTP endpoint mocking

	"github.com/mintel/elasticsearch-asg/internal/pkg/testutil"
)

func TestCheckLiveHEAD_passing(t *testing.T) {
	defer setTestTimeout()
	ctx, _, teardown := testutil.ClientTestSetup(t)
	defer teardown()
	const u = "http://127.0.0.1:9200"

	check := CheckLiveHEAD(ctx, u)
	gock.New(u).
		Head("/").
		Reply(http.StatusOK)
	err := check()
	assert.NoError(t, err)
	assert.True(t, gock.IsDone())
}

func TestCheckLiveHEAD_error(t *testing.T) {
	defer setTestTimeout()
	ctx, _, teardown := testutil.ClientTestSetup(t)
	defer teardown()
	const u = "http://127.0.0.1:9200"

	check := CheckLiveHEAD(ctx, u)
	gock.New(u).
		Head("/").
		Reply(http.StatusInternalServerError).
		BodyString(http.StatusText(http.StatusInternalServerError))
	err := check()
	assert.Error(t, err)
	assert.True(t, gock.IsDone())
}
