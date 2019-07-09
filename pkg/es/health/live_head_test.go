package health

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckLiveHEAD_passing(t *testing.T) {
	check, _, mux, teardown := setup(t, CheckLiveHEAD)
	defer teardown()
	mux.On("HEAD", "/", nil, nil).Once().Return(http.StatusOK, nil, nil)
	err := check()
	assert.NoError(t, err)
	mux.AssertExpectations(t)
}

func TestCheckLiveHEAD_timeout(t *testing.T) {
	check, _, mux, teardown := setup(t, CheckLiveHEAD)
	defer teardown()
	mux.On("HEAD", "/", nil, nil).Once().After(DefaultHTTPTimeout*2).Return(http.StatusOK, nil, nil)
	err := check()
	assert.Error(t, err)
	mux.AssertExpectations(t)
}

func TestCheckLiveHEAD_error(t *testing.T) {
	check, _, mux, teardown := setup(t, CheckLiveHEAD)
	defer teardown()
	mux.On("HEAD", "/", nil, nil).Once().Return(http.StatusInternalServerError, nil, http.StatusText(http.StatusInternalServerError))
	err := check()
	assert.Error(t, err)
	mux.AssertExpectations(t)
}
