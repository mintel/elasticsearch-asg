// Package mockhttp extends https://godoc.org/github.com/stretchr/testify/mock
// for mocking http servers.
//
//   func TestMockRESTAPI(t *testing.T) {
//   	mux := &mockhttp.Mux{} // Used to mock HTTP endpoints
//   	server := httptest.NewServer(mux)
//   	defer server.Close()
//
//   	client := myapi.Client(server.URL)
//   	p := &myapi.Person{
//   		ID:   1,
//   		Name: "Testy McTestface",
//   	}
//
//   	mux.On("POST", "/foobar", nil, p).Once().Return(http.StatusCreated, nil, nil)
//   	err := client.PostThing(1, "Testy McTestface")
//   	assert.NoError(t, err)
//
//   	mux.On("GET", "/foobar", nil, nil).Once().Return(http.StatusOK, nil, p)
//   	result, err := client.GetThing()
//   	assert.NoError(t, err)
//   	assert.Equal(t, p, result)
//
//   	mux.AssertExpectations(t) // Assert all expected endpoint calls were made
//   }
//
package mockhttp

import (
	"encoding/json"
	"net/http"

	"github.com/stretchr/testify/mock"
)

// Mux is a http.Handler for mocking server endpoints.
type Mux struct {
	mock.Mock
}

// On starts a description of an expectation of the specified endpoint being called.
//
// Body may be one of: nil, a byte slice, a string, or a JSON-encodable struct.
func (m *Mux) On(method, path string, h http.Header, body interface{}) *Call {
	return newCall(m, method, path, h, body)
}

func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	args := m.MethodCalled(methodName(r.Method, r.URL.Path), w, r)
	status := args.Int(0)
	header := args.Get(1).(http.Header)
	body := args.Get(2)

	// Write headers
	if header != nil {
		wHeader := w.Header()
		for k := range wHeader { // Suppress default headers
			wHeader[k] = nil
		}
		wHeader["Content-Type"] = []string{defaultContentType(body)}
		for k, v := range header {
			wHeader[k] = v
		}
	}
	w.WriteHeader(status)

	// Write body
	if bodyData, err := bodyBytes(body); err != nil {
		panic(err)
	} else if _, err = w.Write(bodyData); err != nil {
		panic(err)
	}
}

// Assert interface.
var _ http.Handler = (*Mux)(nil)

func methodName(method, path string) string {
	return method + " " + path
}

func defaultContentType(body interface{}) string {
	const (
		d = "application/octet-stream" // http://www.w3.org/Protocols/rfc2616/rfc2616-sec7.html#sec7.2.1
		j = "application/json; charset=utf-8"
	)
	switch b := body.(type) {
	case string:
		if json.Valid([]byte(b)) {
			return j
		}
		return d
	case []byte:
		if json.Valid(b) {
			return j
		}
		return d
	case nil:
		return d
	default:
		return j
	}
}
