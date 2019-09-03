package elasticsearch

import (
	"strings"

	"github.com/tidwall/gjson"
)

// shardAllocationExcludeSettings represents the transient shard allocation exclusions
// of an Elasticsearch cluster.
type shardAllocationExcludeSettings struct {
	Name, Host, IP []string
	Attr           map[string][]string
}

// newShardAllocationExcludeSettings creates a new shardAllocationExcludeSettings.
func newShardAllocationExcludeSettings(settings *gjson.Result) *shardAllocationExcludeSettings {
	s := &shardAllocationExcludeSettings{
		Attr: make(map[string][]string),
	}
	settings.Get(shardAllocExcludeSetting).ForEach(func(key, value gjson.Result) bool {
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

func (s *shardAllocationExcludeSettings) Map() map[string]*string {
	m := make(map[string]*string)
	if s.Name != nil {
		if len(s.Name) == 0 {
			m[shardAllocExcludeSetting+"._name"] = nil
		} else {
			m[shardAllocExcludeSetting+"._name"] = strPtr(strings.Join(s.Name, ","))
		}
	}
	if s.Host != nil {
		if len(s.Host) == 0 {
			m[shardAllocExcludeSetting+"._host"] = nil
		} else {
			m[shardAllocExcludeSetting+"._host"] = strPtr(strings.Join(s.Host, ","))
		}
	}
	if s.IP != nil {
		if len(s.IP) == 0 {
			m[shardAllocExcludeSetting+"._ip"] = nil
		} else {
			m[shardAllocExcludeSetting+"._ip"] = strPtr(strings.Join(s.IP, ","))
		}
	}
	for k, v := range s.Attr {
		if len(v) == 0 {
			m[shardAllocExcludeSetting+"."+k] = nil
		} else {
			m[shardAllocExcludeSetting+"."+k] = strPtr(strings.Join(v, ","))
		}
	}
	return m
}

func strPtr(s string) *string {
	return &s
}
