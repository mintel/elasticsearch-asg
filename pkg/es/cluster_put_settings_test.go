package es

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mintel/elasticsearch-asg/internal/pkg/testutil"
	"github.com/mintel/elasticsearch-asg/pkg/ctxlog"
)

func TestClusterPutSettingsService(t *testing.T) {
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

	resp, err := NewClusterPutSettingsService(client).Transient("cluster.routing.allocation.exclude._name", "foo").Persistent("cluster.routing.allocation.exclude._name", "bar").Do(ctx)
	if assert.NoError(t, err) {
		assert.Equal(t, "foo", resp.Transient.Get("cluster.routing.allocation.exclude._name").String())
		assert.Equal(t, "bar", resp.Persistent.Get("cluster.routing.allocation.exclude._name").String())
	}

	resp, err = NewClusterPutSettingsService(client).Transient("cluster.routing.allocation.exclude._name", nil).Persistent("cluster.routing.allocation.exclude._name", nil).Do(ctx)
	if assert.NoError(t, err) {
		assert.Nil(t, resp.Transient.Get("cluster.routing.allocation.exclude._name").Value())
		assert.Nil(t, resp.Persistent.Get("cluster.routing.allocation.exclude._name").Value())
	}
}
