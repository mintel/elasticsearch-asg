package esasg

import (
	"context"
	"sort"
	"strings"
	"sync"

	elastic "github.com/olivere/elastic/v7"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"

	"github.com/mintel/elasticsearch-asg/pkg/es"
)

const shardAllocExcludeSetting = "cluster.routing.allocation.exclude"

// ElasticsearchCommandService describes methods to modify Elasticsearch.
type ElasticsearchCommandService interface {
	// Drain excludes a node from shard allocation, which will cause Elasticsearch
	// to remove shards from the node until empty.
	//
	// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/allocation-filtering.html
	Drain(ctx context.Context, nodeName string) error

	// Undrain reverses Drain.
	//
	// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/allocation-filtering.html
	Undrain(ctx context.Context, nodeName string) error

	// ExcludeMasterVoting excludes a node from voting in master elections
	// or being eligible to be the master node.
	// It will return an error if the specified node doesn't have the "master" role.
	//
	// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/voting-config-exclusions.html
	ExcludeMasterVoting(ctx context.Context, nodeName string) error

	// ClearMasterVotingExclusions removes all master voting exclusions.
	//
	// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/voting-config-exclusions.html
	ClearMasterVotingExclusions(ctx context.Context) error
}

// elasticsearchCommandService implements the ElasticsearchCommandService interface.
type elasticsearchCommandService struct {
	ElasticsearchCommandService

	client     *elastic.Client
	logger     *zap.Logger
	settingsMu sync.Mutex // Elasticsearch doesn't provide an atomic way to modify settings
}

// NewElasticsearchCommandService returns a new ElasticsearchCommandService.
func NewElasticsearchCommandService(client *elastic.Client) ElasticsearchCommandService {
	return &elasticsearchCommandService{
		client: client,
		logger: zap.L().Named("elasticsearchCommandService"),
	}
}

// Drain excludes a node from shard allocation, which will cause Elasticsearch
// to remove shards from the node until empty.
//
// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/allocation-filtering.html
func (s *elasticsearchCommandService) Drain(ctx context.Context, nodeName string) error {
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()

	resp, err := es.NewClusterGetSettingsService(s.client).Do(ctx)
	if err != nil {
		return err
	}

	settings := newshardAllocationExcludeSettings(resp.Transient)
	settings.Name = append(settings.Name, nodeName)
	sort.Strings(settings.Name)
	// ignore everything but name
	settings.IP = nil
	settings.Host = nil
	settings.Attr = nil

	_, err = es.NewClusterPutSettingsService(s.client).BodyJSON(map[string]interface{}{"transient": settings.Map()}).Do(ctx)
	return err
}

// Undrain reverses Drain.
//
// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/allocation-filtering.html
func (s *elasticsearchCommandService) Undrain(ctx context.Context, nodeName string) error {
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()

	resp, err := es.NewClusterGetSettingsService(s.client).Do(ctx)
	if err != nil {
		return err
	}

	settings := newshardAllocationExcludeSettings(resp.Transient)
	found := false
	filtered := settings.Name[:0]
	for _, name := range settings.Name {
		if name == nodeName {
			found = true
		} else {
			filtered = append(filtered, name)
		}
	}
	if !found {
		return nil
	}
	sort.Strings(filtered)
	settings.Name = filtered
	// ignore everything but name
	settings.IP = nil
	settings.Host = nil
	settings.Attr = nil

	_, err = es.NewClusterPutSettingsService(s.client).BodyJSON(map[string]interface{}{"transient": settings.Map()}).Do(ctx)
	return err
}

// ExcludeMasterVoting excludes a node from voting in master elections
// or being eligible to be the master node.
// It will return an error if the specified node doesn't have the "master" role.
//
// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/voting-config-exclusions.html
func (s *elasticsearchCommandService) ExcludeMasterVoting(ctx context.Context, nodeName string) error {
	_, err := es.NewClusterPostVotingConfigExclusion(s.client).Node(nodeName).Do(ctx)
	return err
}

// ClearMasterVotingExclusions removes all master voting exclusions.
//
// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/voting-config-exclusions.html
func (s *elasticsearchCommandService) ClearMasterVotingExclusions(ctx context.Context) error {
	_, err := es.NewClusterDeleteVotingConfigExclusion(s.client).Do(ctx)
	return err
}

// shardAllocationExcludeSettings represents the transient shard allocation exclusions
// of an Elasticsearch cluster.
type shardAllocationExcludeSettings struct {
	Name, Host, IP []string
	Attr           map[string][]string
}

// newshardAllocationExcludeSettings creates a new shardAllocationExcludeSettings.
func newshardAllocationExcludeSettings(settings *gjson.Result) *shardAllocationExcludeSettings {
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

func (s *shardAllocationExcludeSettings) Map() map[string]interface{} {
	m := make(map[string]interface{})
	if s.Name != nil {
		if len(s.Name) == 0 {
			m[shardAllocExcludeSetting+"._name"] = nil
		} else {
			m[shardAllocExcludeSetting+"._name"] = strings.Join(s.Name, ",")
		}
	}
	if s.Host != nil {
		if len(s.Host) == 0 {
			m[shardAllocExcludeSetting+"._host"] = nil
		} else {
			m[shardAllocExcludeSetting+"._host"] = strings.Join(s.Host, ",")
		}
	}
	if s.IP != nil {
		if len(s.IP) == 0 {
			m[shardAllocExcludeSetting+"._ip"] = nil
		} else {
			m[shardAllocExcludeSetting+"._ip"] = strings.Join(s.IP, ",")
		}
	}
	for k, v := range s.Attr {
		if len(v) == 0 {
			m[shardAllocExcludeSetting+"."+k] = nil
		} else {
			m[shardAllocExcludeSetting+"."+k] = strings.Join(v, ",")
		}
	}
	return m
}
