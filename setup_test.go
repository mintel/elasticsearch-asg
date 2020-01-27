package esasg

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// setup sets up zap test logging. It returns a
// suitable URL for mock endpoints and a teardown function.
func setup(t *testing.T) (string, func()) {
	logger := zaptest.NewLogger(t)
	f1 := zap.ReplaceGlobals(logger)
	f2 := zap.RedirectStdLog(logger)
	teardown := func() {
		f2()
		f1()
		_ = logger.Sync()
	}
	return "http://127.0.0.1:9200", teardown
}

// loadTestData is help to load test data from the `testdata` directory.
func loadTestData(t *testing.T, name string) string {
	path := filepath.Join("testdata", name) // relative path
	data, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to load test data file %s: %s", name, err)
	}
	return string(data)
}
