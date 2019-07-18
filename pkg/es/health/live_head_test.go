package health

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert" // Test assertion e.g. equality
	gock "gopkg.in/h2non/gock.v1"        // HTTP endpoint mocking
)

func TestCheckLiveHEAD_passing(t *testing.T) {
	u, teardown := setup(t)
	defer teardown()
	defer gock.Off()
	check := CheckLiveHEAD(u)
	gock.New(u).
		Head("/").
		Reply(http.StatusOK)
	err := check()
	assert.NoError(t, err)
	assert.True(t, gock.IsDone())
}

func TestCheckLiveHEAD_error(t *testing.T) {
	u, teardown := setup(t)
	defer teardown()
	defer gock.Off()
	check := CheckLiveHEAD(u)
	gock.New(u).
		Head("/").
		Reply(http.StatusInternalServerError).
		BodyString(http.StatusText(http.StatusInternalServerError))
	err := check()
	assert.Error(t, err)
	assert.True(t, gock.IsDone())
}
