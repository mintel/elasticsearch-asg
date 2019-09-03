package elasticsearch

import (
	"context"
	"net/http"
	"testing"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client
	"github.com/stretchr/testify/assert"    // Test assertions e.g. equality
	gock "gopkg.in/h2non/gock.v1"           // HTTP endpoint mocking
)

const (
	node1Name          = "foo"
	node2Name          = "bar"
	sortedNodeNameList = "bar,foo"
)

// b is a quick and dirty map type for specifying JSON bodies.
type b map[string]interface{}

func TestCommand_Drain(t *testing.T) {
	u, teardown := setup(t)
	defer teardown()

	defer gock.OffAll()
	// gock.Observe(gock.DumpRequest) // Log HTTP requests during test.

	// Drain a node.
	gock.New(u).
		Get("/_cluster/settings").
		Reply(http.StatusOK).
		JSON(b{"persistent": b{}, "transient": b{}})
	gock.New(u).
		Put("/_cluster/settings").
		JSON(b{"transient": b{"cluster.routing.allocation.exclude._name": node1Name}}).
		Reply(http.StatusOK).
		JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node1Name}}}}}})
	esClient, err := elastic.NewSimpleClient(elastic.SetURL(u))
	if err != nil {
		t.Fatalf("couldn't create elastic client: %s", err)
	}
	s := NewCommand(esClient)
	err = s.Drain(context.Background(), node1Name)
	assert.NoError(t, err)
	assert.True(t, gock.IsDone())

	// Draining the same node again shouldn't need to PUT any settings.
	gock.New(u).
		Get("/_cluster/settings").
		Reply(http.StatusOK).
		JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node1Name}}}}}})
	err = s.Drain(context.Background(), node1Name)
	assert.NoError(t, err)
	assert.True(t, gock.IsDone())

	// Draining a second node results in a comma-separated list in sorted order.
	gock.New(u).
		Get("/_cluster/settings").
		Reply(http.StatusOK).
		JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node1Name}}}}}})
	gock.New(u).
		Put("/_cluster/settings").
		JSON(b{"transient": b{"cluster.routing.allocation.exclude._name": sortedNodeNameList}}).
		Reply(http.StatusOK).
		JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": sortedNodeNameList}}}}}})
	err = s.Drain(context.Background(), node2Name)
	assert.NoError(t, err)
	assert.True(t, gock.IsDone())
}

func TestCommand_Undrain(t *testing.T) {
	u, teardown := setup(t)
	defer teardown()

	defer gock.OffAll()
	// gock.Observe(gock.DumpRequest) // Log HTTP requests during test.

	// Undrain a node.
	gock.New(u).
		Get("/_cluster/settings").
		Reply(http.StatusOK).
		JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": sortedNodeNameList}}}}}})
	gock.New(u).
		Put("/_cluster/settings").
		JSON(b{"transient": b{"cluster.routing.allocation.exclude._name": node2Name}}).
		Reply(http.StatusOK).
		JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node2Name}}}}}})
	esClient, err := elastic.NewSimpleClient(elastic.SetURL(u))
	if err != nil {
		t.Fatalf("couldn't create elastic client: %s", err)
	}
	s := NewCommand(esClient)
	err = s.Undrain(context.Background(), node1Name)
	assert.NoError(t, err)
	assert.True(t, gock.IsDone())

	// Undrain last node.
	gock.New(u).
		Get("/_cluster/settings").
		Reply(http.StatusOK).
		JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node2Name}}}}}})
	gock.New(u).
		Put("/_cluster/settings").
		JSON(b{"transient": b{"cluster.routing.allocation.exclude._name": nil}}).
		Reply(http.StatusOK).
		JSON(b{"persistent": b{}, "transient": b{}})
	err = s.Undrain(context.Background(), node2Name)
	assert.NoError(t, err)
	assert.True(t, gock.IsDone())
}
