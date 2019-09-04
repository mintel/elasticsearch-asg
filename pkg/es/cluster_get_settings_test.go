package es

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mintel/elasticsearch-asg/internal/pkg/testutil"
	"github.com/mintel/elasticsearch-asg/pkg/ctxlog"
)

func TestClusterGetSettingsService(t *testing.T) {
	logger, teardownLogging := testutil.TestLogger(t)
	defer teardownLogging()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = ctxlog.WithLogger(ctx, logger)

	container, client, err := runElasticsearch(t)
	if err != nil {
		t.Fatal(err)
	}
	defer container.Close()

	resp, err := NewClusterGetSettingsService(client).Defaults(true).Do(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.Defaults.Map())
}
