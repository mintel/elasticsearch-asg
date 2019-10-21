package es

import (
	"testing"

	"github.com/stretchr/testify/assert" // Test assertions e.g. equality.
	"github.com/tidwall/gjson"           // Dynamic JSON parsing.

	"github.com/mintel/elasticsearch-asg/internal/pkg/testutil" // Testing utilities.
)

func TestNewShardAllocationSettings(t *testing.T) {
	s := loadSettings()

	persistent := NewShardAllocationExcludeSettings(s.Persistent)
	assert.Empty(t, persistent.Host)
	assert.Empty(t, persistent.IP)
	assert.Empty(t, persistent.Attr)
	assert.Empty(t, persistent.Name)

	transient := NewShardAllocationExcludeSettings(s.Transient)
	assert.Empty(t, transient.Host)
	assert.Empty(t, transient.IP)
	assert.Empty(t, transient.Attr)
	assert.Equal(t, []string{"i-0adf68017a253c05d"}, transient.Name)
}

func loadSettings() *ClusterGetSettingsResponse {
	data := testutil.LoadTestData("shard_allocation_settings.json")
	result := gjson.ParseBytes([]byte(data))
	persistent := result.Get("persistent")
	transient := result.Get("transient")
	return &ClusterGetSettingsResponse{
		Persistent: &persistent,
		Transient:  &transient,
	}
}
