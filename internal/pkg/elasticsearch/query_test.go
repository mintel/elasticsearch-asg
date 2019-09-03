package elasticsearch

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client
	"github.com/stretchr/testify/assert"    // Test assertions e.g. equality
	gock "gopkg.in/h2non/gock.v1"           // HTTP endpoint mocking
)

func TestQuery_ClusterName(t *testing.T) {
	gock.Intercept()
	defer gock.OffAll()
	// gock.Observe(gock.DumpRequest) // Log HTTP requests during test.

	u, teardown := setup(t)
	defer teardown()

	esClient, err := elastic.NewSimpleClient(elastic.SetURL(u))
	if err != nil {
		t.Fatalf("couldn't create elastic client: %s", err)
	}
	q := NewQuery(esClient)

	const want = "mycluster"

	gock.New(u).
		Get("/_cluster/health").
		Reply(http.StatusOK).
		JSON(b{
			"active_primary_shards":            0,
			"active_shards":                    0,
			"active_shards_percent_as_number":  100.0,
			"cluster_name":                     want,
			"delayed_unassigned_shards":        0,
			"initializing_shards":              0,
			"number_of_data_nodes":             1,
			"number_of_in_flight_fetch":        0,
			"number_of_nodes":                  1,
			"number_of_pending_tasks":          0,
			"relocating_shards":                0,
			"status":                           "green",
			"task_max_waiting_in_queue_millis": 0,
			"timed_out":                        false,
			"unassigned_shards":                0,
		})
	got, err := q.ClusterName(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, want, got)
	assert.True(t, gock.IsDone())
}

func TestQuery_ClusterHealth(t *testing.T) {
	gock.Intercept()
	defer gock.OffAll()
	// gock.Observe(gock.DumpRequest) // Log HTTP requests during test.

	u, teardown := setup(t)
	defer teardown()

	esClient, err := elastic.NewSimpleClient(elastic.SetURL(u))
	if err != nil {
		t.Fatalf("couldn't create elastic client: %s", err)
	}
	q := NewQuery(esClient)

	const clusterName = "mycluster"

	gock.New(u).
		Get("/_cluster/health").
		Reply(http.StatusOK).
		JSON(b{
			"active_primary_shards":            0,
			"active_shards":                    0,
			"active_shards_percent_as_number":  100.0,
			"cluster_name":                     clusterName,
			"delayed_unassigned_shards":        0,
			"initializing_shards":              0,
			"number_of_data_nodes":             1,
			"number_of_in_flight_fetch":        0,
			"number_of_nodes":                  1,
			"number_of_pending_tasks":          0,
			"relocating_shards":                0,
			"status":                           "green",
			"task_max_waiting_in_queue_millis": 0,
			"timed_out":                        false,
			"unassigned_shards":                0,
		})
	health, err := q.ClusterHealth(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, clusterName, health.ClusterName)
	assert.True(t, gock.IsDone())
}

func TestQuery_Nodes(t *testing.T) {
	gock.Intercept()
	defer gock.OffAll()
	// gock.Observe(gock.DumpRequest) // Log HTTP requests during test.

	u, teardown := setup(t)
	defer teardown()

	esClient, err := elastic.NewSimpleClient(elastic.SetURL(u))
	if err != nil {
		t.Fatalf("couldn't create elastic client: %s", err)
	}
	q := NewQuery(esClient)

	gock.New(u).
		Get("/_nodes/stats").
		Reply(200).
		Type("json").
		BodyString(loadTestData(t, "nodes_stats.json"))
	gock.New(u).
		Get("/_nodes/_all/_all").
		Reply(200).
		Type("json").
		BodyString(loadTestData(t, "nodes_info.json"))
	gock.New(u).
		Get("/_cluster/settings").
		Reply(200).
		Type("json").
		BodyString(loadTestData(t, "cluster_settings.json"))
	gock.New(u).
		Get("/_cat/shards").
		Reply(200).
		Type("json").
		BodyString(loadTestData(t, "cat_shards.json"))

	nodes, err := q.Nodes(context.Background())
	assert.NoError(t, err)
	assert.True(t, gock.IsDone())
	assert.Len(t, nodes, 9)
}

func TestQuery_Node(t *testing.T) {
	gock.Intercept()
	defer gock.OffAll()
	// gock.Observe(gock.DumpRequest) // Log HTTP requests during test.

	u, teardown := setup(t)
	defer teardown()

	const nodeName = "i-0f5c6d4d61d41b9fc"

	esClient, err := elastic.NewSimpleClient(elastic.SetURL(u))
	if err != nil {
		t.Fatalf("couldn't create elastic client: %s", err)
	}
	q := NewQuery(esClient)

	gock.New(u).
		Get(fmt.Sprintf("/_nodes/%s/stats", nodeName)).
		Reply(http.StatusOK).
		Type("json").
		BodyString(loadTestData(t, "nodes_stats_"+nodeName+".json"))
	gock.New(u).
		Get(fmt.Sprintf("/_nodes/%s/_all", nodeName)).
		Reply(http.StatusOK).
		Type("json").
		BodyString(loadTestData(t, "nodes_info_"+nodeName+".json"))
	gock.New(u).
		Get("/_cluster/settings").
		Reply(http.StatusOK).
		Type("json").
		BodyString(loadTestData(t, "cluster_settings.json"))
	gock.New(u).
		Get("/_cat/shards").
		Reply(http.StatusOK).
		Type("json").
		BodyString(loadTestData(t, "cat_shards.json"))

	n, err := q.Node(context.Background(), nodeName)
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

func TestQuery_GetSnapshots(t *testing.T) {
	gock.Intercept()
	defer gock.OffAll()
	// gock.Observe(gock.DumpRequest) // Log HTTP requests during test.

	u, teardown := setup(t)
	defer teardown()

	const repoName = "myrepo"

	esClient, err := elastic.NewSimpleClient(elastic.SetURL(u))
	if err != nil {
		t.Fatalf("couldn't create elastic client: %s", err)
	}
	q := NewQuery(esClient)

	t.Run("all", func(t *testing.T) {
		gock.New(u).
			Get(fmt.Sprintf("/_snapshot/%s/_all", repoName)).
			Reply(http.StatusOK).
			Type("json").
			BodyString(loadTestData(t, "snapshots_get_all.json"))
		snapshots, err := q.GetSnapshots(context.Background(), repoName)
		assert.NoError(t, err)
		assert.True(t, gock.IsDone())
		assert.NotNil(t, snapshots)
	})

	t.Run("some", func(t *testing.T) {
		const (
			snapshot1 = "analytics-20190902t040001"
			snapshot2 = "analytics-20190903t040001"
		)
		gock.New(u).
			Get(fmt.Sprintf("/_snapshot/%s/%s,%s", repoName, snapshot1, snapshot2)).
			Reply(http.StatusOK).
			Type("json").
			BodyString(loadTestData(t, "snapshots_get_some.json"))
		snapshots, err := q.GetSnapshots(context.Background(), repoName, snapshot1, snapshot2)
		assert.NoError(t, err)
		assert.True(t, gock.IsDone())
		assert.Len(t, snapshots, 2)
	})
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
