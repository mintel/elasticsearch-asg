package es

import (
	"context"
	"testing"

	elastic "github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"
)

func TestCatShardsService(t *testing.T) {
	defer setupLogging(t)()

	client, err := elastic.NewClient()
	if !assert.NoError(t, err) {
		return
	}
	defer client.Stop()

	ctx := context.Background()

	_, err = client.CreateIndex("foobar").Do(ctx)
	if !assert.NoError(t, err) {
		return
	}
	defer client.DeleteIndex("foobar").Do(ctx)

	resp, err := NewCatShardsService(client).Columns("*").Do(ctx)
	if assert.NoError(t, err) && assert.NotEmpty(t, resp) {
		assert.NotEqual(t, "", resp[0].ID)
	}
}
