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
		Old *ClusterState
		New *ClusterState
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
				Old: &ClusterState{
					Nodes: []string{"a"},
				},
				New: &ClusterState{
					Nodes: []string{"a", "b", "c"},
				},
			},
			wantAdd:    []string{"b", "c"},
			wantRemove: []string{},
		},
		{
			name: "all-remove",
			args: args{
				Old: &ClusterState{
					Nodes: []string{"a", "b", "c", "d"},
				},
				New: &ClusterState{
					Nodes: []string{"a", "d"},
				},
			},
			wantAdd:    []string{},
			wantRemove: []string{"b", "c"},
		},
		{
			name: "same",
			args: args{
				Old: &ClusterState{
					Nodes: []string{"a", "b", "c"},
				},
				New: &ClusterState{
					Nodes: []string{"a", "b", "c"},
				},
			},
			wantAdd:    []string{},
			wantRemove: []string{},
		},
		{
			name: "diff",
			args: args{
				Old: &ClusterState{
					Nodes: []string{"a", "b", "c"},
				},
				New: &ClusterState{
					Nodes: []string{"a", "b", "d"},
				},
			},
			wantAdd:    []string{"d"},
			wantRemove: []string{"c"},
		},
		{
			name: "old-nil",
			args: args{
				Old: nil,
				New: &ClusterState{
					Nodes: []string{"a", "b", "d"},
				},
			},
			wantAdd:    []string{"a", "b", "d"},
			wantRemove: []string{},
		},
		{
			name: "new-nil",
			args: args{
				Old: &ClusterState{
					Nodes: []string{"a", "b", "c"},
				},
				New: nil,
			},
			wantAdd:    []string{},
			wantRemove: []string{"a", "b", "c"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.True(t, tt.args.Old == nil || sort.StringsAreSorted(tt.args.Old.Nodes))
			require.True(t, tt.args.New == nil || sort.StringsAreSorted(tt.args.New.Nodes))
			gotAdd, gotRemove := tt.args.Old.DiffNodes(tt.args.New)
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
				Old: &ClusterState{
					Nodes: []string{"b", "a"},
				},
				New: &ClusterState{
					Nodes: []string{"a", "b"},
				},
			},
		},
		{
			name: "panic-new",
			args: args{
				Old: &ClusterState{
					Nodes: []string{"a", "b"},
				},
				New: &ClusterState{
					Nodes: []string{"b", "a"},
				},
			},
		},
		{
			name: "panic-both",
			args: args{
				Old: &ClusterState{
					Nodes: []string{"b", "a"},
				},
				New: &ClusterState{
					Nodes: []string{"b", "a"},
				},
			},
		},
	}
	for _, tt := range panicTests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Panics(t, func() {
				tt.args.Old.DiffNodes(tt.args.New)
			})
		})
	}
}

func TestClusterState_DiffShards(t *testing.T) {
	type args struct {
		Old *ClusterState
		New *ClusterState
	}
	tests := []struct {
		name string
		args args
		want map[string]int
	}{
		{
			name: "add",
			args: args{
				Old: &ClusterState{
					Shards: map[string]int{"a": 0, "b": 1},
				},
				New: &ClusterState{
					Shards: map[string]int{"a": 1, "b": 2},
				},
			},
			want: map[string]int{"a": 1, "b": 1},
		},
		{
			name: "remove",
			args: args{
				Old: &ClusterState{
					Shards: map[string]int{"a": 1, "b": 2},
				},
				New: &ClusterState{
					Shards: map[string]int{"a": 0, "b": 0},
				},
			},
			want: map[string]int{"a": -1, "b": -2},
		},
		{
			name: "both",
			args: args{
				Old: &ClusterState{
					Shards: map[string]int{"a": 1, "b": 4},
				},
				New: &ClusterState{
					Shards: map[string]int{"a": 2, "b": 3},
				},
			},
			want: map[string]int{"a": 1, "b": -1},
		},
		{
			name: "same",
			args: args{
				Old: &ClusterState{
					Shards: map[string]int{"a": 1, "b": 2},
				},
				New: &ClusterState{
					Shards: map[string]int{"a": 1, "b": 2},
				},
			},
			want: map[string]int{"a": 0, "b": 0},
		},
		{
			name: "nil-old",
			args: args{
				Old: nil,
				New: &ClusterState{
					Shards: map[string]int{"a": 1, "b": 2},
				},
			},
			want: map[string]int{"a": 1, "b": 2},
		},
		{
			name: "nil-new",
			args: args{
				Old: &ClusterState{
					Shards: map[string]int{"a": 1, "b": 2},
				},
				New: nil,
			},
			want: map[string]int{"a": -1, "b": -2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.args.Old.DiffShards(tt.args.New)
			assert.Equal(t, tt.want, got)
		})
	}
}
