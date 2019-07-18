package health

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	gock "gopkg.in/h2non/gock.v1"
)

func TestCheckLiveHEAD_passing(t *testing.T) {
	check, teardown := setup(t, CheckLiveHEAD)
	defer teardown()
	defer gock.Off()
	gock.New(localhost).
		Head("/").
		Reply(http.StatusOK)
	err := check()
	assert.NoError(t, err)
	assert.True(t, gock.IsDone())
}

func TestCheckLiveHEAD_error(t *testing.T) {
	check, teardown := setup(t, CheckLiveHEAD)
	defer teardown()
	defer gock.Off()
	gock.New(localhost).
		Head("/").
		Reply(http.StatusInternalServerError).
		BodyString(http.StatusText(http.StatusInternalServerError))
	err := check()
	assert.Error(t, err)
	assert.True(t, gock.IsDone())
}
