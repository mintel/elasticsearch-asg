package mockhttp

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/stretchr/testify/mock"
)

// Call represent an expectation that an HTTP endpoint will be called.
type Call struct {
	*mock.Call

	Parent *Mux
	Method string
	Path   string
	Header http.Header
	Body   interface{}
}

func newCall(parent *Mux, method, path string, h http.Header, body interface{}) *Call {
	if method == "" {
		method = "GET"
	}
	c := &Call{
		Parent: parent,
		Method: method,
		Path:   path,
		Header: h,
		Body:   body,
	}
	name := methodName(method, path)
	c.Call = parent.Mock.On(name, mock.MatchedBy(matchResponseWriter), mock.MatchedBy(c.requestMatch))
	return c
}

// Run sets a handler to be called before returning.
func (c *Call) Run(fn func(http.ResponseWriter, *http.Request)) *Call {
	c.Call = c.Call.Run(func(args mock.Arguments) {
		w := args[0].(http.ResponseWriter)
		r := args[1].(*http.Request)
		fn(w, r)
	})
	return c
}

// Return specifies the return arguments for the expectation.
func (c *Call) Return(status int, h http.Header, body interface{}) *Call {
	c.Call = c.Call.Return(status, h, body)
	return c
}

// On chains a new expectation description onto the mocked endpoint.
//
// This allows syntax like:
//   Mux.
//     On("POST", "/foo", nil, v).ReturnJSON(http.StatusCreated, nil, nil).
//     On("GET", "/foo", nil, nil).ReturnJSON(http.StatusOK, nil, v)
func (c *Call) On(method, path string, h http.Header, body interface{}) *Call {
	return c.Parent.On(method, path, h, body)
}

// Once indicates that that the mock should only return the value once.
func (c *Call) Once() *Call {
	c.Call = c.Call.Once()
	return c
}

// Twice indicates that that the mock should only return the value twice.
func (c *Call) Twice() *Call {
	c.Call = c.Call.Twice()
	return c
}

// Times indicates that that the mock should only return the indicated number
// of times.
func (c *Call) Times(i int) *Call {
	c.Call = c.Call.Times(i)
	return c
}

// WaitUntil sets the channel that will block the mock's return until its closed
// or a message is received.
func (c *Call) WaitUntil(w <-chan time.Time) *Call {
	c.Call = c.Call.WaitUntil(w)
	return c
}

// After sets how long to block until the call returns
func (c *Call) After(d time.Duration) *Call {
	c.Call = c.Call.After(d)
	return c
}

// Maybe allows the method call to be optional. Not calling an optional method
// will not cause an error while asserting expectations
func (c *Call) Maybe() *Call {
	c.Call = c.Call.Maybe()
	return c
}

// requestMatch returns true if an http.Request matches the Call args.
func (c *Call) requestMatch(r *http.Request) bool {
	if r.Method != c.Method {
		return false
	}

	if r.URL.Path != c.Path {
		return false
	}

	for k, v := range c.Header {
		if rv, ok := r.Header[k]; !ok || !headersEqual(v, rv) {
			return false
		}
	}

	// Check the request body.
	// mock will end up calling requestMatch multiple times, so
	// first make sure GetBody is set so we can read the body multiple times.
	if r.GetBody == nil {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			panic(err)
		}
		r.GetBody = func() (io.ReadCloser, error) {
			return ioutil.NopCloser(bytes.NewReader(body)), nil
		}
	}

	if bodyReader, err := r.GetBody(); err != nil {
		panic(err)
	} else if body, err := ioutil.ReadAll(bodyReader); err != nil {
		panic(err)
	} else if match, err := bodiesEqual(body, c.Body); err != nil {
		panic(err)
	} else {
		return match
	}
}

func matchResponseWriter(w http.ResponseWriter) bool {
	return true
}

// headersEqual returns true if two slices of strings contain the same unique set of strings (any order).
func headersEqual(a, b []string) bool {
	am := make(map[string]struct{}, len(a))
	for _, s := range a {
		am[s] = struct{}{}
	}
	bm := make(map[string]struct{}, len(b))
	for _, s := range b {
		bm[s] = struct{}{}
	}
	if len(am) != len(bm) {
		return false
	}
	for s := range am {
		if _, ok := bm[s]; !ok {
			return false
		}
	}
	for s := range bm {
		if _, ok := am[s]; !ok {
			return false
		}
	}
	return true
}
