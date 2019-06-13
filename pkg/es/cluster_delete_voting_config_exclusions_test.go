package es

import (
	"context"
	"testing"

	elastic "github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"
)

func TestClusterDeleteVotingConfigExclusion(t *testing.T) {
	defer setupLogging(t)()

	client, err := elastic.NewClient()
	if !assert.NoError(t, err) {
		return
	}
	defer client.Stop()

	ctx := context.Background()
	_, err = NewClusterDeleteVotingConfigExclusion(client).Do(ctx)
	assert.NoError(t, err)
}
