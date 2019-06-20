package esasg

import (
	"context"
	"testing"

	elastic "github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestElasticsearchQueryService_Nodes(t *testing.T) {
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
	s := NewElasticsearchQueryService(client)

	ctx := context.Background()
	nodes, err := s.Nodes(ctx)
	if assert.NoError(t, err) && assert.Len(t, nodes, 1) {
		var name string
		var node *Node
		for name, node = range nodes {
			break
		}
		assert.Equal(t, name, node.Name)
		assert.Equal(t, "elasticsearch", node.ClusterName)
		assert.NotNil(t, node.JVM)
	}
}

func TestElasticsearchQueryService_Node(t *testing.T) {
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
	s := NewElasticsearchQueryService(client)

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
