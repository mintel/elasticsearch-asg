package esasg

import (
	"context"
	"net/http"
	"testing"

	elastic "github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"
	gock "gopkg.in/h2non/gock.v1"
)

const (
	localhost          = "http://127.0.0.1:9200"
	clusterSettingsAPI = "/_cluster/settings"
	node1Name          = "foo"
	node2Name          = "bar"
	sortedNodeNameList = "bar,foo"
)

type b map[string]interface{} // a quick and dirty map type for specifying JSON bodies.

func TestElasticsearchCommandService_Drain(t *testing.T) {
	defer setupLogging(t)()
	defer gock.OffAll()
	// gock.Observe(gock.DumpRequest) // Log HTTP requests during test.

	esClient, err := elastic.NewSimpleClient(elastic.SetURL(localhost))
	if err != nil {
		t.Fatalf("couldn't create elastic client: %s", err)
	}
	s := NewElasticsearchCommandService(esClient)

	// Drain a node.
	gock.New(localhost).
		Get(clusterSettingsAPI).
		Reply(http.StatusOK).
		JSON(b{"persistent": b{}, "transient": b{}})
	gock.New(localhost).
		Put(clusterSettingsAPI).
		JSON(b{"transient": b{"cluster.routing.allocation.exclude._name": node1Name}}).
		Reply(http.StatusOK).
		JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node1Name}}}}}})
	err = s.Drain(context.Background(), node1Name)
	assert.NoError(t, err)
	assert.True(t, gock.IsDone())

	// Draining the same node again shouldn't need to PUT any settings.
	gock.New(localhost).
		Get(clusterSettingsAPI).
		Reply(http.StatusOK).
		JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node1Name}}}}}})
	err = s.Drain(context.Background(), node1Name)
	assert.NoError(t, err)
	assert.True(t, gock.IsDone())

	// Draining a second node results in a comma-separated list in sorted order.
	gock.New(localhost).
		Get(clusterSettingsAPI).
		Reply(http.StatusOK).
		JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node1Name}}}}}})
	gock.New(localhost).
		Put(clusterSettingsAPI).
		JSON(b{"transient": b{"cluster.routing.allocation.exclude._name": sortedNodeNameList}}).
		Reply(http.StatusOK).
		JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": sortedNodeNameList}}}}}})
	err = s.Drain(context.Background(), node2Name)
	assert.NoError(t, err)
	assert.True(t, gock.IsDone())
}

func TestElasticsearchCommandService_Undrain(t *testing.T) {
	defer setupLogging(t)()
	defer gock.OffAll()
	// gock.Observe(gock.DumpRequest) // Log HTTP requests during test.

	esClient, err := elastic.NewSimpleClient(elastic.SetURL(localhost))
	if err != nil {
		t.Fatalf("couldn't create elastic client: %s", err)
	}
	s := NewElasticsearchCommandService(esClient)

	// Undrain a node.
	gock.New(localhost).
		Get(clusterSettingsAPI).
		Reply(http.StatusOK).
		JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": sortedNodeNameList}}}}}})
	gock.New(localhost).
		Put(clusterSettingsAPI).
		JSON(b{"transient": b{"cluster.routing.allocation.exclude._name": node2Name}}).
		Reply(http.StatusOK).
		JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node2Name}}}}}})
	err = s.Undrain(context.Background(), node1Name)
	assert.NoError(t, err)
	assert.True(t, gock.IsDone())

	// Undrain last node.
	gock.New(localhost).
		Get(clusterSettingsAPI).
		Reply(http.StatusOK).
		JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node2Name}}}}}})
	gock.New(localhost).
		Put(clusterSettingsAPI).
		JSON(b{"transient": b{"cluster.routing.allocation.exclude._name": nil}}).
		Reply(http.StatusOK).
		JSON(b{"persistent": b{}, "transient": b{}})
	err = s.Undrain(context.Background(), node2Name)
	assert.NoError(t, err)
	assert.True(t, gock.IsDone())
}
