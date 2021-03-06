package es

import (
	"context"
	"net/http"
	"testing"

	"github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"

	gock "gopkg.in/h2non/gock.v1"
)

func TestClusterPostVotingConfigExclusions(t *testing.T) {
	// This test just mocks the Elasticsearch endpoint instead of
	// running a Docker container. This API endpoint returns an error
	// if Elasticsearch is running in single-node mode, so we'd have to
	// run a whole cluster.
	// TODO: Docker integration tests with a whole cluster.
	const (
		localhost = "http://127.0.0.1:9200"
		nodeName  = "foobar"
	)
	defer gock.Off()
	gock.New(localhost).
		Post("/_cluster/voting_config_exclusions/" + nodeName).
		Reply(http.StatusAccepted)
	client, err := elastic.NewSimpleClient(elastic.SetURL(localhost))
	if !assert.NoError(t, err) {
		return
	}
	_, err = NewClusterPostVotingConfigExclusion(client).Node(nodeName).Do(context.Background())
	assert.NoError(t, err)
	assert.True(t, gock.IsDone())
}
