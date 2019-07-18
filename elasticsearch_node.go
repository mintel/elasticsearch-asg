package esasg

import (
	"sort"

	elastic "github.com/olivere/elastic/v7"

	"github.com/mintel/elasticsearch-asg/pkg/es"
)

// Node represents info and stats about an Elasticsearch node at a point in time.
type Node struct {
	elastic.NodesInfoNode

	ClusterName             string
	ElectedMaster           bool
	ExcludedShardAllocation bool
	Stats                   elastic.NodesStatsNode
	Shards                  es.CatShardsResponse
}

// Indices returns list of of index names present on this shard.
func (n *Node) Indices() []string {
	m := make(map[string]struct{})
	for _, s := range n.Shards {
		m[s.Index] = struct{}{}
	}
	indices := make([]string, 0, len(m))
	for i := range m {
		indices = append(indices, i)
	}
	sort.Strings(indices)
	return indices
}
