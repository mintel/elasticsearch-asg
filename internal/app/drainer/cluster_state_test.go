package drainer

import (
	"sort"
	"testing"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client.
	"github.com/stretchr/testify/assert"    // Test assertions e.g. equality.
	"github.com/stretchr/testify/require"   // Like assert but fails the test.
	"github.com/tidwall/gjson"              // Dynamic JSON parsing.

	"github.com/mintel/elasticsearch-asg/pkg/es" // Extensions to the Elasticsearch client.
)

func TestNewClusterState(t *testing.T) {
	i := &elastic.NodesInfoResponse{
		Nodes: map[string]*elastic.NodesInfoNode{
			"uhfpaiewfjjahef": &elastic.NodesInfoNode{
				Name: "i-abc123",
			},
			"flajguhtaijdfes": &elastic.NodesInfoNode{
				Name: "i-def456",
			},
		},
	}
	s := es.CatShardsResponse{
		es.CatShardsResponseRow{
			Node: strPtr("i-def456"),
		},
		es.CatShardsResponseRow{
			Node: strPtr("i-abc123 -> 10.2.4.58 kNe49LLvSqGXBn2s8Ffgyw i-def456"),
		},
	}
	te := gjson.Parse(`{"cluster": {"routing": {"allocation": {"exclude": {"_name": "i-abc123"}}}}}`)
	set := &es.ClusterGetSettingsResponse{
		Transient: &te,
	}
	want := &ClusterState{
		Nodes: []string{"i-abc123", "i-def456"},
		Shards: map[string]int{
			"i-abc123": 1,
			"i-def456": 2,
		},
		Exclusions: &es.ShardAllocationExcludeSettings{
			Name: []string{"i-abc123"},
			Attr: make(map[string][]string),
		},
	}
	got := NewClusterState(i, s, set)
	assert.Equal(t, want, got)
}

func TestClusterState_DiffNodes(t *testing.T) {
	type args struct {
		OldNodes []string
		NewNodes []string
	}
	tests := []struct {
		name       string
		args       args
		wantAdd    []string
		wantRemove []string
	}{
		{
			name: "all-add",
			args: args{
				OldNodes: []string{"a"},
				NewNodes: []string{"a", "b", "c"},
			},
			wantAdd:    []string{"b", "c"},
			wantRemove: []string{},
		},
		{
			name: "all-remove",
			args: args{
				OldNodes: []string{"a", "b", "c", "d"},
				NewNodes: []string{"a", "d"},
			},
			wantAdd:    []string{},
			wantRemove: []string{"b", "c"},
		},
		{
			name: "same",
			args: args{
				OldNodes: []string{"a", "b", "c"},
				NewNodes: []string{"a", "b", "c"},
			},
			wantAdd:    []string{},
			wantRemove: []string{},
		},
		{
			name: "diff",
			args: args{
				OldNodes: []string{"a", "b", "c"},
				NewNodes: []string{"a", "b", "d"},
			},
			wantAdd:    []string{"d"},
			wantRemove: []string{"c"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.True(t, sort.StringsAreSorted(tt.args.OldNodes))
			require.True(t, sort.StringsAreSorted(tt.args.NewNodes))
			s := &ClusterState{
				Nodes: tt.args.OldNodes,
			}
			o := &ClusterState{
				Nodes: tt.args.NewNodes,
			}
			gotAdd, gotRemove := s.DiffNodes(o)
			assert.ElementsMatch(t, tt.wantAdd, gotAdd)
			assert.ElementsMatch(t, tt.wantRemove, gotRemove)
		})
	}

	panicTests := []struct {
		name string
		args args
	}{
		{
			name: "panic-old",
			args: args{
				OldNodes: []string{"b", "a"},
				NewNodes: []string{"a", "b"},
			},
		},
		{
			name: "panic-new",
			args: args{
				OldNodes: []string{"a", "b"},
				NewNodes: []string{"b", "a"},
			},
		},
		{
			name: "panic-both",
			args: args{
				OldNodes: []string{"b", "a"},
				NewNodes: []string{"b", "a"},
			},
		},
	}
	for _, tt := range panicTests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Panics(t, func() {
				s := &ClusterState{
					Nodes: tt.args.OldNodes,
				}
				o := &ClusterState{
					Nodes: tt.args.NewNodes,
				}
				s.DiffNodes(o)
			})
		})
	}
}

func TestClusterState_DiffShards(t *testing.T) {
	type args struct {
		OldShards map[string]int
		NewShards map[string]int
	}
	tests := []struct {
		name string
		args args
		want map[string]int
	}{
		{
			name: "add",
			args: args{
				OldShards: map[string]int{"a": 0, "b": 1},
				NewShards: map[string]int{"a": 1, "b": 2},
			},
			want: map[string]int{"a": 1, "b": 1},
		},
		{
			name: "remove",
			args: args{
				OldShards: map[string]int{"a": 1, "b": 2},
				NewShards: map[string]int{"a": 0, "b": 0},
			},
			want: map[string]int{"a": -1, "b": -2},
		},
		{
			name: "both",
			args: args{
				OldShards: map[string]int{"a": 1, "b": 4},
				NewShards: map[string]int{"a": 2, "b": 3},
			},
			want: map[string]int{"a": 1, "b": -1},
		},
		{
			name: "same",
			args: args{
				OldShards: map[string]int{"a": 1, "b": 2},
				NewShards: map[string]int{"a": 1, "b": 2},
			},
			want: map[string]int{"a": 0, "b": 0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ClusterState{
				Shards: tt.args.OldShards,
			}
			o := &ClusterState{
				Shards: tt.args.NewShards,
			}
			got := s.DiffShards(o)
			assert.Equal(t, tt.want, got)
		})
	}
}
