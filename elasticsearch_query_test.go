package esasg

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"testing"

	elastic "github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"
	gock "gopkg.in/h2non/gock.v1"
)

func TestElasticsearchQueryService_Nodes(t *testing.T) {
	defer setupLogging(t)()
	defer gock.OffAll()
	// gock.Observe(gock.DumpRequest) // Log HTTP requests during test.

	esClient, err := elastic.NewSimpleClient(elastic.SetURL(testhost))
	if err != nil {
		t.Fatalf("couldn't create elastic client: %s", err)
	}
	s := NewElasticsearchQueryService(esClient)

	gock.New(testhost).
		Get("/_nodes/stats").
		Reply(200).
		Type("json").
		BodyString(helperLoadTestData(t, "nodes_stats.json"))
	gock.New(testhost).
		Get("/_nodes/_all/_all").
		Reply(200).
		Type("json").
		BodyString(helperLoadTestData(t, "nodes_info.json"))
	gock.New(testhost).
		Get("/_cluster/settings").
		Reply(200).
		Type("json").
		BodyString(helperLoadTestData(t, "cluster_settings.json"))
	gock.New(testhost).
		Get("/_cat/shards").
		Reply(200).
		Type("json").
		BodyString(helperLoadTestData(t, "cat_shards.json"))

	nodes, err := s.Nodes(context.Background())
	assert.NoError(t, err)
	assert.True(t, gock.IsDone())
	assert.Len(t, nodes, 9)
}

func TestElasticsearchQueryService_Node(t *testing.T) {
	defer setupLogging(t)()
	defer gock.OffAll()
	// gock.Observe(gock.DumpRequest) // Log HTTP requests during test.

	esClient, err := elastic.NewSimpleClient(elastic.SetURL(testhost))
	if err != nil {
		t.Fatalf("couldn't create elastic client: %s", err)
	}
	s := NewElasticsearchQueryService(esClient)

	const (
		nodeName = "i-0f5c6d4d61d41b9fc"
	)

	gock.New(testhost).
		Get("/_nodes/" + nodeName + "/stats").
		Reply(200).
		Type("json").
		BodyString(helperLoadTestData(t, "nodes_stats_"+nodeName+".json"))
	gock.New(testhost).
		Get("/_nodes/" + nodeName + "/_all").
		Reply(200).
		Type("json").
		BodyString(helperLoadTestData(t, "nodes_info_"+nodeName+".json"))
	gock.New(testhost).
		Get("/_cluster/settings").
		Reply(200).
		Type("json").
		BodyString(helperLoadTestData(t, "cluster_settings.json"))
	gock.New(testhost).
		Get("/_cat/shards").
		Reply(200).
		Type("json").
		BodyString(helperLoadTestData(t, "cat_shards.json"))

	n, err := s.Node(context.Background(), nodeName)
	assert.NoError(t, err)
	assert.True(t, gock.IsDone())
	assert.NotNil(t, n)
	assert.Equal(t, nodeName, n.Name)
	assert.Equal(t, []string{"data"}, n.Roles)
	assert.Equal(t, map[string]string{
		"aws_availability_zone":  "us-east-2a",
		"aws_instance_type":      "i3.large",
		"aws_instance_lifecycle": "spot",
		"xpack.installed":        "true",
		"aws_instance_family":    "i3",
	}, n.Attributes)
	assert.Len(t, n.Shards, 1)
}

func TestParseShardNodes(t *testing.T) {
	testCases := []struct {
		desc  string
		input string
		want  []string
		err   bool
	}{
		{
			desc:  "unassigned-shard",
			input: "",
			want:  nil,
		},
		{
			desc:  "shard",
			input: "i-0968d7621b79cd73d",
			want:  []string{"i-0968d7621b79cd73d"},
		},
		{
			desc:  "relocating-shard",
			input: "i-0968d7621b79cd73d -> 10.2.4.58 kNe49LLvSqGXBn2s8Ffgyw i-0a2ed08df0e5cfff6",
			want:  []string{"i-0968d7621b79cd73d", "i-0a2ed08df0e5cfff6"},
		},
		{
			desc:  "error",
			input: "not a node",
			err:   true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			got, err := parseShardNodes(tC.input)
			assert.Equal(t, got, tC.want)
			if tC.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func helperLoadTestData(t *testing.T, name string) string {
	path := filepath.Join("testdata", name) // relative path
	data, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to load test data file %s: %s", name, err)
	}
	return string(data)
}
