package es

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCatShardsService(t *testing.T) {
	container, client, err := runElasticsearch(t)
	if err != nil {
		t.Fatal(err)
	}
	defer container.Close()

	ctx := context.Background()

	_, err = client.CreateIndex("foobar").Do(ctx)
	if !assert.NoError(t, err) {
		return
	}

	resp, err := NewCatShardsService(client).Columns("*").Do(ctx)
	if assert.NoError(t, err) && assert.NotEmpty(t, resp) {
		assert.NotEqual(t, "", resp[0].ID)
	}
}
