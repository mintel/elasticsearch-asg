package es

import (
	"context"
	elastic "github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestClusterPutSettingsService(t *testing.T) {
	defer setupLogging(t)()

	client, err := elastic.NewClient()
	if !assert.NoError(t, err) {
		return
	}
	defer client.Stop()

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
