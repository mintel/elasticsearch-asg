package esasg

import (
	"context"
	"testing"

	elastic "github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/mintel/elasticsearch-asg/pkg/es"
)

func setupElasticsearchService(options ...elastic.ClientOptionFunc) (*ElasticsearchService, func()) {
	client, err := elastic.NewClient(options...)
	if err != nil {
		zap.L().Panic("couldn't create elastic client", zap.Error(err))
	}
	return NewElasticsearchService(client), client.Stop
}

func TestElasticsearchService_Nodes(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	defer setupLogging(t)()
	es, teardown := setupElasticsearchService()
	defer teardown()

	ctx := context.Background()
	nodes, err := es.Nodes(ctx)
	if assert.NoError(t, err) && assert.Len(t, nodes, 1) {
		var name string
		var node *Node
		for name, node = range nodes {
		}
		assert.Equal(t, name, node.Name)
		assert.Equal(t, "elasticsearch", node.ClusterName)
		assert.NotNil(t, node.JVM)
	}
}

func TestElasticsearchService_Node(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	defer setupLogging(t)()
	s, teardown := setupElasticsearchService()
	defer teardown()

	ctx := context.Background()
	nodes, err := s.Nodes(ctx)
	var name string
	var expected *Node
	for name, expected = range nodes {
		break
	}
	if assert.NoError(t, err) && assert.Len(t, nodes, 1) {
		n, err := s.Node(ctx, name)
		assert.NoError(t, err)
		assert.Equal(t, expected.Name, n.Name)
	}
}

func TestElasticsearchService_Drain(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	defer setupLogging(t)()
	s, teardown := setupElasticsearchService()
	defer teardown()

	ctx := context.Background()

	n1 := &Node{NodesInfoNode: elastic.NodesInfoNode{Name: "foo"}}
	n2 := &Node{NodesInfoNode: elastic.NodesInfoNode{Name: "bar"}}

	assert.NoError(t, s.Drain(ctx, n1))
	resp, err := es.NewClusterGetSettingsService(s.client).Do(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "foo", resp.Transient.Get("cluster.routing.allocation.exclude._name").String())

	assert.NoError(t, s.Drain(ctx, n2))
	resp, err = es.NewClusterGetSettingsService(s.client).Do(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "bar,foo", resp.Transient.Get("cluster.routing.allocation.exclude._name").String())

	assert.NoError(t, s.Undrain(ctx, n1))
	resp, err = es.NewClusterGetSettingsService(s.client).Do(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "bar", resp.Transient.Get("cluster.routing.allocation.exclude._name").String())

	assert.NoError(t, s.Undrain(ctx, n2))
	resp, err = es.NewClusterGetSettingsService(s.client).Do(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "", resp.Transient.Get("cluster.routing.allocation.exclude._name").String())
}
