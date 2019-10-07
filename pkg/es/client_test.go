package es

import (
	"net/http"
	"testing"
	"time"

	elastic "github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"
	gock "gopkg.in/h2non/gock.v1"

	"github.com/mintel/elasticsearch-asg/internal/pkg/testutil" // Testing utilities.
)

func TestDialContextRetry(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		ctx, _, teardown := testutil.ClientTestSetup(t)
		defer teardown()

		c, err := DialContextRetry(
			ctx, time.Millisecond, 20*time.Millisecond,
			elastic.SetSniff(false),
		)
		assert.Nil(t, c)
		assert.Error(t, err)
		assert.Condition(t, gock.IsDone)
	})

	t.Run("ok", func(t *testing.T) {
		ctx, _, teardown := testutil.ClientTestSetup(t)
		defer teardown()

		// The client constructor does two health checks
		// in a row.
		gock.New(elastic.DefaultURL).
			Head("").
			Reply(http.StatusOK)
		gock.New(elastic.DefaultURL).
			Head("").
			Reply(http.StatusOK)

		c, err := DialContextRetry(
			ctx, time.Millisecond, 20*time.Millisecond,
			elastic.SetSniff(false),
		)
		assert.NotNil(t, c)
		assert.NoError(t, err)
		assert.Condition(t, gock.IsDone)
	})
}
