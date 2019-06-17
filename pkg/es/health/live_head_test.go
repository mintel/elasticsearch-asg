package health

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCheckLiveHEAD_passing(t *testing.T) {
	check, _, mux, teardown := setup(t, CheckLiveHEAD)
	defer teardown()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			panic("got non-HEAD request")
		}
		if r.URL.Path != "/" {
			panic("got non-root URL")
		}
		w.WriteHeader(http.StatusOK)
	})
	err := check()
	assert.NoError(t, err)
}

func TestCheckLiveHEAD_timeout(t *testing.T) {
	check, _, mux, teardown := setup(t, CheckLiveHEAD)
	defer teardown()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			panic("got non-HEAD request")
		}
		if r.URL.Path != "/" {
			panic("got non-root URL")
		}
		time.Sleep(DefaultHTTPTimeout * 2)
		w.WriteHeader(http.StatusOK)
	})
	err := check()
	assert.Error(t, err)
}

func TestCheckLiveHEAD_error(t *testing.T) {
	check, _, mux, teardown := setup(t, CheckLiveHEAD)
	defer teardown()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			panic("got non-HEAD request")
		}
		if r.URL.Path != "/" {
			panic("got non-root URL")
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
	err := check()
	assert.Error(t, err)
}
