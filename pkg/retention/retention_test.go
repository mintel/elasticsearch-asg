package retention

import (
	"testing"

	"github.com/stretchr/testify/assert" // Test assertions e.g. equality.
)

func TestKeep(t *testing.T) {
	t.Run("canonical", func(t *testing.T) {
		c, input, _, _, want, _ := canonical()
		got := Keep(c, input)
		assert.Equal(t, want, got)
	})
}

func TestDelete(t *testing.T) {
	t.Run("canonical", func(t *testing.T) {
		c, input, _, _, _, want := canonical()
		got := Delete(c, input)
		assert.Equal(t, want, got)
	})
}

func Test_redistributeBackups(t *testing.T) {
	t.Run("canonical", func(t *testing.T) {
		c, times, initial, want, _, _ := canonical()
		buckets := newBuckets(c, times[len(times)-1])
		buckets.Assign(times)
		assert.Equal(t, initial, buckets)
		redistributeBackups(buckets)
		assert.Equal(t, want, buckets)
	})
}
