package mockhttp

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var client = &http.Client{
	Timeout: time.Second,
}

func setup() (*httptest.Server, *Mux, func()) {
	m := &Mux{}
	server := httptest.NewServer(m)
	return server, m, server.Close
}

func TestMux(t *testing.T) {
	testCases := []struct {
		desc, method, path    string
		status                int
		reqHeader, respHeader http.Header
		reqBody, respBody     interface{}
		f                     func(*Call) *Call
	}{
		{
			desc:     "get-foobar",
			method:   "GET",
			path:     "/foobar",
			status:   http.StatusOK,
			reqBody:  nil,
			respBody: `{"name": "baz"}`,
		},
		{
			desc:     "post-foobar",
			method:   "POST",
			path:     "/foobar",
			status:   http.StatusCreated,
			reqBody:  `{"name": "baz"}`,
			respBody: nil,
		},
		{
			desc:   "body-struct",
			method: "POST",
			path:   "/foobar",
			status: http.StatusCreated,
			reqBody: &Body{
				X: 1,
				Y: "foobar",
				z: true,
			},
			respBody: nil,
		},
		{
			desc:       "headers",
			method:     "GET",
			path:       "/foobar",
			status:     http.StatusOK,
			reqHeader:  http.Header{"Accept": []string{"application/json"}},
			respHeader: http.Header{"Content-Type": []string{"application/json"}},
			reqBody:    nil,
			respBody:   `{"name": "baz"}`,
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			server, mux, teardown := setup()
			defer teardown()
			call := mux.On(tC.method, tC.path, tC.reqHeader, tC.reqBody)
			if tC.f != nil {
				call = tC.f(call)
			}
			call.Return(tC.status, tC.respHeader, tC.respBody)
			b, err := bodyBytes(tC.reqBody)
			if !assert.NoError(t, err) {
				return
			}
			u, err := url.Parse(server.URL)
			if !assert.NoError(t, err) {
				return
			}
			u, err = u.Parse(tC.path)
			if !assert.NoError(t, err) {
				return
			}
			req, err := http.NewRequest(tC.method, u.String(), bytes.NewReader(b))
			for k, v := range tC.reqHeader {
				req.Header[k] = v
			}
			if !assert.NoError(t, err) {
				return
			}
			resp, err := client.Do(req)
			if assert.NoError(t, err) {
				mux.AssertExpectations(t)
				want, err := bodyBytes(tC.respBody)
				if !assert.NoError(t, err) {
					return
				}
				got, err := ioutil.ReadAll(resp.Body)
				if !assert.NoError(t, err) {
					return
				}
				assert.Equal(t, string(want), string(got))
			}
		})
	}
}
