package es

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClusterPutSettingsService(t *testing.T) {
	container, client, err := runElasticsearch(t)
	if err != nil {
		t.Fatal(err)
	}
	defer container.Close()

	ctx := context.Background()

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
