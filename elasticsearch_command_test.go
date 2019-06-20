package esasg

import (
	"context"
	"testing"

	elastic "github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/mintel/elasticsearch-asg/pkg/es"
)

func TestElasticsearchCommandService_Drain(t *testing.T) {
	// This test uses Elasticsearch run by docker-compose.
	// Skip for faster testing.
	if testing.Short() {
		t.SkipNow()
	}

	defer setupLogging(t)()

	client, err := elastic.NewSimpleClient()
	if err != nil {
		zap.L().Panic("couldn't create elastic client", zap.Error(err))
	}
	s := NewElasticsearchCommandService(client)

	ctx := context.Background()

	n1 := &Node{NodesInfoNode: elastic.NodesInfoNode{Name: "foo"}}
	n2 := &Node{NodesInfoNode: elastic.NodesInfoNode{Name: "bar"}}

	assert.NoError(t, s.Drain(ctx, n1.Name))
	resp, err := es.NewClusterGetSettingsService(client).Do(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "foo", resp.Transient.Get("cluster.routing.allocation.exclude._name").String())

	assert.NoError(t, s.Drain(ctx, n2.Name))
	resp, err = es.NewClusterGetSettingsService(client).Do(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "bar,foo", resp.Transient.Get("cluster.routing.allocation.exclude._name").String())

	assert.NoError(t, s.Undrain(ctx, n1.Name))
	resp, err = es.NewClusterGetSettingsService(client).Do(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "bar", resp.Transient.Get("cluster.routing.allocation.exclude._name").String())

	assert.NoError(t, s.Undrain(ctx, n2.Name))
	resp, err = es.NewClusterGetSettingsService(client).Do(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "", resp.Transient.Get("cluster.routing.allocation.exclude._name").String())
}
