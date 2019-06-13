package health

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestCheckLiveHEAD_passing(t *testing.T) {
	client, _, mux, teardown := setup(t)
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
	err := CheckLiveHEAD(context.TODO(), client, zap.L().Named("head"))
	assert.NoError(t, err)
}

func TestCheckLiveHEAD_timeout(t *testing.T) {
	client, _, mux, teardown := setup(t)
	defer teardown()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			panic("got non-HEAD request")
		}
		if r.URL.Path != "/" {
			panic("got non-root URL")
		}
		time.Sleep(timeout * 2)
		w.WriteHeader(http.StatusOK)
	})
	err := CheckLiveHEAD(context.TODO(), client, zap.L().Named("head"))
	assert.Error(t, err)
}

func TestCheckLiveHEAD_error(t *testing.T) {
	client, _, mux, teardown := setup(t)
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
	err := CheckLiveHEAD(context.TODO(), client, zap.L().Named("head"))
	assert.Error(t, err)
}
