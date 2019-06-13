package es

import (
	"context"
	elastic "github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestClusterGetSettingsService(t *testing.T) {
	defer setupLogging(t)()

	client, err := elastic.NewClient()
	if !assert.NoError(t, err) {
		return
	}
	defer client.Stop()

	ctx := context.Background()
	resp, err := NewClusterGetSettingsService(client).Defaults(true).Do(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.Defaults.Map())
}
