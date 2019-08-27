package es

import (
	"strings"

	"github.com/tidwall/gjson"
)

const (
	// ShardAllocExcludeSetting is the JSON path to the shard
	// allocation exclusions in the settings returned by
	// the Elasticsearch GET /_cluster/settings API.
	ShardAllocExcludeSetting = "cluster.routing.allocation.exclude"
)

// ShardAllocationExcludeSettings represents the shard allocation
// exclusion settings of an Elasticsearch cluster.
type ShardAllocationExcludeSettings struct {
	Name, Host, IP []string
	Attr           map[string][]string
}

// NewShardAllocationExcludeSettings creates a new shardAllocationExcludeSettings.
func NewShardAllocationExcludeSettings(settings *gjson.Result) *ShardAllocationExcludeSettings {
	s := &ShardAllocationExcludeSettings{
		Attr: make(map[string][]string),
	}
	settings.Get(ShardAllocExcludeSetting).ForEach(func(key, value gjson.Result) bool {
		k := key.String()
		v := strings.Split(value.String(), ",")
		switch k {
		case "_name":
			s.Name = v
		case "_ip":
			s.IP = v
		case "_host":
			s.Host = v
		default:
			s.Attr[k] = v
		}
		return true
	})
	return s
}

func (s *ShardAllocationExcludeSettings) Map() map[string]*string {
	m := make(map[string]*string)
	if s.Name != nil {
		if len(s.Name) == 0 {
			m[ShardAllocExcludeSetting+"._name"] = nil
		} else {
			m[ShardAllocExcludeSetting+"._name"] = strPtr(strings.Join(s.Name, ","))
		}
	}
	if s.Host != nil {
		if len(s.Host) == 0 {
			m[ShardAllocExcludeSetting+"._host"] = nil
		} else {
			m[ShardAllocExcludeSetting+"._host"] = strPtr(strings.Join(s.Host, ","))
		}
	}
	if s.IP != nil {
		if len(s.IP) == 0 {
			m[ShardAllocExcludeSetting+"._ip"] = nil
		} else {
			m[ShardAllocExcludeSetting+"._ip"] = strPtr(strings.Join(s.IP, ","))
		}
	}
	for k, v := range s.Attr {
		if len(v) == 0 {
			m[ShardAllocExcludeSetting+"."+k] = nil
		} else {
			m[ShardAllocExcludeSetting+"."+k] = strPtr(strings.Join(v, ","))
		}
	}
	return m
}
