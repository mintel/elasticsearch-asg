package es

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mintel/elasticsearch-asg/internal/pkg/testutil"
	"github.com/mintel/elasticsearch-asg/pkg/ctxlog"
)

func TestCatShardsService(t *testing.T) {
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

	_, err = client.CreateIndex("foobar").Do(ctx)
	if !assert.NoError(t, err) {
		return
	}

	resp, err := NewCatShardsService(client).Columns("*").Do(ctx)
	if assert.NoError(t, err) && assert.NotEmpty(t, resp) {
		assert.NotEqual(t, "", resp[0].ID)
	}
}
