package es

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClusterDeleteVotingConfigExclusion(t *testing.T) {
	container, client, err := runElasticsearch(t)
	if err != nil {
		t.Fatal(err)
	}
	defer container.Close()

	ctx := context.Background()
	_, err = NewClusterDeleteVotingConfigExclusion(client).Do(ctx)
	assert.NoError(t, err)
}
