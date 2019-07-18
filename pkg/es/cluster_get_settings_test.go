package es

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClusterGetSettingsService(t *testing.T) {
	container, client, err := runElasticsearch(t)
	if err != nil {
		t.Fatal(err)
	}
	defer container.Close()

	ctx := context.Background()
	resp, err := NewClusterGetSettingsService(client).Defaults(true).Do(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.Defaults.Map())
}
