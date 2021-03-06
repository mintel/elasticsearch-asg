package esasg

import (
	"context"
	"sort"
	"strings"
	"sync"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client
	"github.com/tidwall/gjson"              // Just-In-Time JSON parsing
	"go.uber.org/zap"                       // Logging

	"github.com/mintel/elasticsearch-asg/pkg/es" // Elasticsearch client extensions
)

const shardAllocExcludeSetting = "cluster.routing.allocation.exclude"

// ElasticsearchCommandService implements methods that write to Elasticsearch endpoints.
type ElasticsearchCommandService struct {
	client     *elastic.Client
	logger     *zap.Logger
	settingsMu sync.Mutex // Elasticsearch doesn't provide an atomic way to modify settings
}

// NewElasticsearchCommandService returns a new ElasticsearchCommandService.
func NewElasticsearchCommandService(client *elastic.Client) *ElasticsearchCommandService {
	return &ElasticsearchCommandService{
		client: client,
		logger: zap.L().Named("ElasticsearchCommandService"),
	}
}

// Drain excludes a node from shard allocation, which will cause Elasticsearch
// to remove shards from the node until empty.
//
// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/allocation-filtering.html
func (s *ElasticsearchCommandService) Drain(ctx context.Context, nodeName string) error {
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()

	resp, err := es.NewClusterGetSettingsService(s.client).Do(ctx)
	if err != nil {
		return err
	}

	settings := newShardAllocationExcludeSettings(resp.Transient)
	sort.Strings(settings.Name)
	i := sort.SearchStrings(settings.Name, nodeName)            // Index in sorted slice where nodeName should be.
	if i < len(settings.Name) && settings.Name[i] == nodeName { // Node is already excluded from allocation.
		return nil
	}
	// Insert nodeName into slice (https://github.com/golang/go/wiki/SliceTricks#insert)
	settings.Name = append(settings.Name, "")
	copy(settings.Name[i+1:], settings.Name[i:])
	settings.Name[i] = nodeName

	// Ignore all node exclusion attributes other than node name.
	settings.IP = nil
	settings.Host = nil
	settings.Attr = nil

	// Update cluster settings with new shard allocation exclusions.
	settingsMap := settings.Map()
	body := map[string]map[string]*string{"transient": settingsMap}
	_, err = es.NewClusterPutSettingsService(s.client).BodyJSON(body).Do(ctx)
	return err
}

// Undrain reverses Drain.
//
// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/allocation-filtering.html
func (s *ElasticsearchCommandService) Undrain(ctx context.Context, nodeName string) error {
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()

	resp, err := es.NewClusterGetSettingsService(s.client).Do(ctx)
	if err != nil {
		return err
	}

	settings := newShardAllocationExcludeSettings(resp.Transient)
	sort.Strings(settings.Name)
	i := sort.SearchStrings(settings.Name, nodeName)             // Index in sorted slice where nodeName should be.
	if i == len(settings.Name) || settings.Name[i] != nodeName { // Node is already not in the shard allocation exclusion list.
		return nil
	}
	// Remove nodeName from slice (https://github.com/golang/go/wiki/SliceTricks#delete)
	settings.Name = settings.Name[:i+copy(settings.Name[i:], settings.Name[i+1:])]

	// Ignore all node exclusion attributes other than node name.
	settings.IP = nil
	settings.Host = nil
	settings.Attr = nil

	// Update cluster settings with new shard allocation exclusions.
	settingsMap := settings.Map()
	body := map[string]map[string]*string{"transient": settingsMap}
	_, err = es.NewClusterPutSettingsService(s.client).BodyJSON(body).Do(ctx)
	return err
}

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
