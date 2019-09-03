package elasticsearch

import (
	"context"
	"sort"
	"strings"
	"sync"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/tidwall/gjson" // Just-In-Time JSON parsing
	"go.uber.org/zap"          // Logging

	"github.com/mintel/elasticsearch-asg/internal/pkg/metrics" // Prometheus metrics
	"github.com/mintel/elasticsearch-asg/pkg/ctxlog"           // Logger from context
	"github.com/mintel/elasticsearch-asg/pkg/es"               // Elasticsearch client extensions
)

const (
	shardAllocExcludeSetting = "cluster.routing.allocation.exclude"
	commandSubsystem         = "command"
)

var (
	// CommandDrainDuration is the Prometheus metric for Command.Drain() durations.
	CommandDrainDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: commandSubsystem,
		Name:      "drain_shards_request_duration_seconds",
		Help:      "Requests to drain shards from Elasticsearch node.",
		Buckets:   prometheus.DefBuckets,
	}, []string{metrics.LabelStatus})

	// CommandUndrainDuration is the Prometheus metric for Command.Undrain() durations.
	CommandUndrainDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: commandSubsystem,
		Name:      "undrain_shards_request_duration_seconds",
		Help:      "Requests to undrain shards from Elasticsearch node.",
		Buckets:   prometheus.DefBuckets,
	}, []string{metrics.LabelStatus})
)

// Command implements methods that write to Elasticsearch endpoints.
type Command struct {
	client     *elastic.Client
	settingsMu sync.Mutex // Elasticsearch doesn't provide an atomic way to modify settings
}

// NewCommand returns a new Command.
func NewCommand(client *elastic.Client) *Command {
	return &Command{
		client: client,
	}
}

// Drain excludes a node from shard allocation, which will cause Elasticsearch
// to remove shards from the node until empty.
//
// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/allocation-filtering.html
func (s *Command) Drain(ctx context.Context, nodeName string) (err error) {
	timer := metrics.NewVecTimer(CommandDrainDuration)
	defer timer.ObserveErr(err)

	logger := ctxlog.L(ctx).Named("Command.Drain").With(zap.String("node_name", nodeName))

	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()

	logger.Debug("getting Elasticsearch settings")
	var resp *es.ClusterGetSettingsResponse
	resp, err = es.NewClusterGetSettingsService(s.client).Do(ctx)
	if err != nil {
		logger.Error("error getting Elasticsearch settings", zap.Error(err))
		return err
	}

	settings := newShardAllocationExcludeSettings(resp.Transient)
	sort.Strings(settings.Name)
	i := sort.SearchStrings(settings.Name, nodeName)            // Index in sorted slice where nodeName should be.
	if i < len(settings.Name) && settings.Name[i] == nodeName { // Node is already excluded from allocation.
		logger.Debug("node already excluded from shard allocation")
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
	if err != nil {
		logger.Error("error putting Elasticsearch settings", zap.Error(err))
	} else {
		logger.Debug("finished putting Elasticsearch settings", zap.Error(err))
	}
	return
}

// Undrain reverses Drain.
//
// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/allocation-filtering.html
func (s *Command) Undrain(ctx context.Context, nodeName string) (err error) {
	timer := metrics.NewVecTimer(CommandUndrainDuration)
	defer timer.ObserveErr(err)

	logger := ctxlog.L(ctx).Named("Command.Undrain").With(zap.String("node_name", nodeName))

	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()

	logger.Debug("getting Elasticsearch settings")
	var resp *es.ClusterGetSettingsResponse
	resp, err = es.NewClusterGetSettingsService(s.client).Do(ctx)
	if err != nil {
		logger.Error("error getting Elasticsearch settings", zap.Error(err))
		return err
	}

	settings := newShardAllocationExcludeSettings(resp.Transient)
	sort.Strings(settings.Name)
	i := sort.SearchStrings(settings.Name, nodeName)             // Index in sorted slice where nodeName should be.
	if i == len(settings.Name) || settings.Name[i] != nodeName { // Node is already not in the shard allocation exclusion list.
		logger.Debug("node already allowed for shard allocation")
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
	if err != nil {
		logger.Error("error putting Elasticsearch settings", zap.Error(err))
	} else {
		logger.Debug("finished putting Elasticsearch settings", zap.Error(err))
	}
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
