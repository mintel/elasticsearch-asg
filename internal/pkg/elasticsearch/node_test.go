package elasticsearch

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert" // Test assertions e.g. equality

	"github.com/mintel/elasticsearch-asg/internal/pkg/testutil"
	"github.com/mintel/elasticsearch-asg/pkg/es" // Elasticsearch client extensions
)

func TestNode_Indices(t *testing.T) {
	data := testutil.LoadTestData("cat_shards.json")
	var shards es.CatShardsResponse
	err := json.Unmarshal([]byte(data), &shards)
	if !assert.NoError(t, err) {
		return
	}
	n := &Node{
		Shards: shards,
	}
	indices := n.Indices()
	assert.Equal(t, []string{".monitoring-es-7-2019.07.18"}, indices)
}
