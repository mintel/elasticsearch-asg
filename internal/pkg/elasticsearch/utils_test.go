package elasticsearch

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"path/filepath"

	"go.uber.org/zap"
	gock "gopkg.in/h2non/gock.v1"
)

// b is a quick and dirty map type for specifying JSON bodies.
type b map[string]interface{}

// gockObserver returns a gock.ObserverFunc that logs HTTP requests to a zap logger.
func gockObserver(logger *zap.Logger) gock.ObserverFunc {
	return func(request *http.Request, mock gock.Mock) {
		bytes, _ := httputil.DumpRequestOut(request, true)
		logger.Debug("gock intercepted http request",
			zap.String("request", string(bytes)),
			zap.Any("matches_mock", mock),
		)
	}
}

// loadTestData is help to load test data from the `testdata` directory.
func loadTestData(name string) string {
	path := filepath.Join("testdata", name) // relative path
	data, err := ioutil.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("failed to load test data file %s: %s", name, err))
	}
	return string(data)
}
