package elasticsearch

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap" // Logging

	"github.com/mintel/elasticsearch-asg/internal/pkg/metrics" // Prometheus metrics
	"github.com/mintel/elasticsearch-asg/pkg/ctxlog"           // Logger from context
	"github.com/mintel/elasticsearch-asg/pkg/es"               // Elasticsearch client extensions
)

const (
	shardAllocExcludeSetting = "cluster.routing.allocation.exclude"
	commandSubsystem         = "command"
)

var (
	// ErrRepoWrongType is returned by Command.EnsureSnapshotRepo() when the
	// repository already exists but is of the wrong type.
	ErrRepoWrongType = errors.New("repository exists but is the wrong type")
)

var (
	// CommandDrainDuration is the Prometheus metric for Command.Drain() durations.
	CommandDrainDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: commandSubsystem,
		Name:      "drain_shards_request_duration_seconds",
		Help:      "Duration requesting to drain shards from an Elasticsearch node.",
		Buckets:   prometheus.DefBuckets,
	}, []string{metrics.LabelStatus})

	// CommandUndrainDuration is the Prometheus metric for Command.Undrain() durations.
	CommandUndrainDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: commandSubsystem,
		Name:      "undrain_shards_request_duration_seconds",
		Help:      "Duration requesting to undrain shards from an Elasticsearch node.",
		Buckets:   prometheus.DefBuckets,
	}, []string{metrics.LabelStatus})

	// CommandEnsureSnapshotRepoDuration is the Prometheus metric for Command.EnsureSnapshotRepo() durations.
	CommandEnsureSnapshotRepoDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: commandSubsystem,
		Name:      "ensure_snapshot_repo_request_duration_seconds",
		Help:      "Duration while ensuring the existence of an Elasticsearch snapshot repository.",
		Buckets:   prometheus.DefBuckets,
	}, []string{metrics.LabelStatus})

	// CommandCreateSnapshotDuration is the Prometheus metric for Command.CreateSnapshot() durations.
	CommandCreateSnapshotDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: commandSubsystem,
		Name:      "create_snapshot_request_duration_seconds",
		Help:      "Duration while creating an Elasticsearch snapshot.",
		Buckets:   prometheus.DefBuckets,
	}, []string{metrics.LabelStatus})

	// CommandDeleteSnapshotDuration is the Prometheus metric for Command.DeleteSnapshot() durations.
	CommandDeleteSnapshotDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: commandSubsystem,
		Name:      "delete_snapshot_request_duration_seconds",
		Help:      "Duration while deleting an Elasticsearch snapshot.",
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
func (cmd *Command) Drain(ctx context.Context, nodeName string) (err error) {
	timer := metrics.NewVecTimer(CommandDrainDuration)
	defer timer.ObserveErr(err)

	logger := ctxlog.L(ctx).Named("Command.Drain").With(zap.String("node_name", nodeName))

	cmd.settingsMu.Lock()
	defer cmd.settingsMu.Unlock()

	logger.Debug("getting Elasticsearch settings")
	var resp *es.ClusterGetSettingsResponse
	resp, err = es.NewClusterGetSettingsService(cmd.client).Do(ctx)
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
	_, err = es.NewClusterPutSettingsService(cmd.client).BodyJSON(body).Do(ctx)
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
func (cmd *Command) Undrain(ctx context.Context, nodeName string) (err error) {
	timer := metrics.NewVecTimer(CommandUndrainDuration)
	defer timer.ObserveErr(err)

	logger := ctxlog.L(ctx).Named("Command.Undrain").With(zap.String("node_name", nodeName))

	cmd.settingsMu.Lock()
	defer cmd.settingsMu.Unlock()

	logger.Debug("getting Elasticsearch settings")
	var resp *es.ClusterGetSettingsResponse
	resp, err = es.NewClusterGetSettingsService(cmd.client).Do(ctx)
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
	_, err = es.NewClusterPutSettingsService(cmd.client).BodyJSON(body).Do(ctx)
	if err != nil {
		logger.Error("error putting Elasticsearch settings", zap.Error(err))
	} else {
		logger.Debug("finished putting Elasticsearch settings", zap.Error(err))
	}
	return err
}

// EnsureSnapshotRepo ensures an Elasticsearch snapshot repository with the given name and type.
//
// If a repository with name doesn't exist, it will be created with the given settings.
// If a repository with name does exist but is the wrong type, ErrRepoWrongType will be returned.
func (cmd *Command) EnsureSnapshotRepo(ctx context.Context, repoName, repoType string, repoSettings map[string]string) (err error) {
	timer := metrics.NewVecTimer(CommandEnsureSnapshotRepoDuration)
	defer timer.ObserveErr(err)

	logger := ctxlog.L(ctx).Named("Command.EnsureSnapshotRepo").With(zap.String("repository_name", repoName))

	logger.Debug("checking for existing repository...")
	var resp elastic.SnapshotGetRepositoryResponse
	resp, err = cmd.client.SnapshotGetRepository(repoName).Do(ctx)

	if err != nil && !elastic.IsNotFound(err) {
		// Unexpected error while checking if snapshot repository exists.
		logger.Error("error checking for existing snapshot repository", zap.Error(err))
	} else if existingRepo, ok := resp[repoName]; elastic.IsNotFound(err) || !ok {
		logger.Debug("repository doesn't exist; creating...")
		s := cmd.client.SnapshotCreateRepository(repoName).Type(repoType)
		for k, v := range repoSettings {
			s = s.Setting(k, v)
		}
		if _, err = s.Do(ctx); err != nil {
			logger.Error("error creating snapshot repository", zap.Error(err))
		}
	} else if ok && existingRepo.Type != repoType {
		// Snapshot repository exists, but is of the wrong type e.g. fs != s3.
		logger.Error(
			"snapshot repository exists, but is the wrong type",
			zap.String("want_type", repoType),
			zap.String("got_type", existingRepo.Type),
		)
		err = ErrRepoWrongType
	} else {
		logger.Debug("repository exists and is the correct type")
	}
	return
}

// CreateSnapshot creates a new Elasticsearch snapshot for the given time.
//
// If now is more than one second greater or less than time.Now(), this func will panic.
func (cmd *Command) CreateSnapshot(ctx context.Context, repoName, format string, now time.Time) (err error) {
	timer := metrics.NewVecTimer(CommandCreateSnapshotDuration)
	defer timer.ObserveErr(err)

	snapshotName := now.Format(format)
	logger := ctxlog.L(ctx).Named("Command.CreateSnapshot").With(
		zap.String("repository_name", repoName),
		zap.Time("now", now),
		zap.String("snapshot_name", snapshotName),
	)

	// Sanity-check now: it should be pretty close to time.Now()
	if d := time.Since(now); -time.Second < d && d < time.Second {
		logger.DPanic("now is not within one second of the current time")
	}
	logger.Info("creating snapshot")
	_, err = cmd.client.SnapshotCreate(repoName, snapshotName).WaitForCompletion(true).Do(ctx)
	if err != nil {
		logger.Error("error creating snapshot", zap.Error(err))
	} else {
		logger.Debug("created snapshot")
	}
	return
}

// DeleteSnapshot deletes the named Elasticsearch snapshot.
// It will block until the snapshot is deleted.
func (cmd *Command) DeleteSnapshot(ctx context.Context, repoName string, snapshotName string) (err error) {
	timer := metrics.NewVecTimer(CommandDeleteSnapshotDuration)
	defer timer.ObserveErr(err)

	logger := ctxlog.L(ctx).Named("Command.DeleteSnapshot").With(
		zap.String("repository_name", repoName),
		zap.String("snapshot_name", snapshotName),
	)

	logger.Debug("deleting snapshot")
	_, err = cmd.client.SnapshotDelete(repoName, snapshotName).Do(ctx)
	if err != nil {
		logger.Error("deleting snapshot", zap.Error(err))
	} else {
		logger.Debug("deleted snapshot")
	}
	return
}
