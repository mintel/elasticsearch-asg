package mockhttp

import (
	"net/http/httptest"
)

// NewServer returns a running test server and mock Mux.
func NewServer() (*httptest.Server, *Mux) {
	mux := &Mux{}
	s := httptest.NewServer(mux)
	return s, mux
}
