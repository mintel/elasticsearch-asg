package main

import (
	"context"
	"time"

	elastic "github.com/olivere/elastic/v7"  // Elasticsearch client
	"go.uber.org/zap"                        // Logging
	kingpin "gopkg.in/alecthomas/kingpin.v2" // Command line args parser

	"github.com/mintel/elasticsearch-asg/cmd" // Common logging setup func
)

const (
	// Initial Elasticsearch exponential backoff retry time.
	esRetryInit = 150 * time.Millisecond
	// Max Elasticsearch exponential backoff retry time.
	esRetryMax = 1200 * time.Millisecond
)

// SnapshotFormat is the format for snapshot names (time.Time.Format()).
// Elasticsearch snapshot names may not contain spaces.
const SnapshotFormat = "2006-01-02-15-04-05"

// defaultURL is the default Elasticsearch URL.
const defaultURL = "http://localhost:9200"

// Command line opts
var (
	esURL        = kingpin.Arg("url", "Elasticsearch URL. Default: "+defaultURL).Default(defaultURL).URL()
	windows      = kingpin.Flag("window", "Snapshot frequency + TTL. May be set multiple times. ISO 8601 Duration string format. Example: `--window P1M=PT1H` == keep hourly snapshots for 1 month.").PlaceHolder("P1M=PT1H").Required().StringMap()
	delete       = kingpin.Flag("delete", "If set, clean up old snapshots. This is false by default for safety's sake.").Short('d').Bool()
	repoName     = kingpin.Flag("repo", "Name of the snapshot repository.").Default("backups").String()
	repoType     = kingpin.Flag("type", "If set, create a repository of this type before creating snapshots. See also: '--settings'").String()
	repoSettings = kingpin.Flag("settings", "Use these settings creating the snapshot repository. May be set multiple times. Example: `--type=s3 --settings bucket=my_bucket`").StringMap()
)

var logger *zap.Logger // XXX: I don't like a global logger var like this. Refactor to derive logger from context.

func main() {
	kingpin.CommandLine.Help = "Create and clean up Elasticsearch snapshots on a schedule."
	kingpin.Parse()

	// Deference global repoName flag pointer to local variable.
	repoName := *repoName

	// Set up logger.
	logger = cmd.SetupLogging().With(zap.String("snapshot_repository", repoName))
	defer func() {
		// Make sure any buffered logs get flushed before exiting successfully.
		// This should never happen because snapshooter should never exit successfully,
		// but just in case...
		// Subsequent calls to loger.Fatal() perform their own Sync().
		// See: https://github.com/uber-go/zap/blob/master/FAQ.md#why-include-dedicated-panic-and-fatal-log-levels
		// Do this inside a closure func so that the linter will stop complaining
		// about not checking the error output of Sync().
		_ = logger.Sync()
	}()

	// Parse the snapshot schedule.
	snapshotSchedule := make(SnapshotWindows, 0)
	for keepFor, every := range *windows {
		w, err := NewSnapshotWindow(every, keepFor)
		if err != nil {
			logger.Fatal("error parsing snapshot window",
				zap.String("keepFor", keepFor),
				zap.String("every", every),
				zap.Error(err),
			)
		}
		snapshotSchedule = append(snapshotSchedule, w)
	}

	ctx := context.Background()

	// Craete Elasticsearch client.
	client, err := elastic.DialContext(
		ctx,
		elastic.SetURL((*esURL).String()),
		elastic.SetRetrier(elastic.NewBackoffRetrier(elastic.NewExponentialBackoff(esRetryInit, esRetryMax))),
	)
	if err != nil {
		logger.Fatal("error creating Elasticsearch client", zap.Error(err))
	}

	// If --type/--settings flags are set, create the snapshot repository if it doesn't exist.
	if repoType != nil && *repoType != "" {
		if err := ensureSnapshotRepo(ctx, client, *repoType, repoName, *repoSettings); err != nil {
			logger.Fatal("error ensuring snapshot repository exists", zap.Error(err))
		}
	}

	for nextSnapshot := snapshotSchedule.Next(); ; nextSnapshot = snapshotSchedule.Next() {
		time.Sleep(time.Until(nextSnapshot)) // Wait to start the snapshot

		// Start a goroutine to create/delete snapshots.
		// Accoring to https://www.elastic.co/guide/en/elasticsearch/reference/7.0/modules-snapshots.html
		//   Only one snapshot process can be executed in the cluster at any time.
		//   While snapshot of a particular shard is being created this shard cannot be moved to another node,
		//   which can interfere with rebalancing process and allocation filtering.
		//   Elasticsearch will only be able to move a shard to another node
		//   (according to the current allocation filtering settings and rebalancing algorithm)
		//   once the snapshot is finished.
		// If this goroutine doesn't finish by the time the next one is started,
		// Elasticsearch will probably return an error and snapshooter will exit.
		go func(t time.Time) {
			logger.Debug("starting snapshot create/delete goroutine")
			if err := createSnapshot(ctx, client, repoName, t); err != nil {
				logger.Fatal("error while creating new snapshot", zap.Error(err))
			}
			if !*delete {
				return // If the --delete flag isn't set, don't clean up old snapshots.
			}
			if err := deleteOldSnapshots(ctx, client, repoName, snapshotSchedule); err != nil {
				logger.Fatal("error while deleting old snapshots", zap.Error(err))
			}
		}(nextSnapshot)
	}
}

// ensureSnapshotRepo ensures an Elasticsearch snapshot repository with the given type, name, and settings exists.
//
// If a repository with name doesn't exist, it will be created.
// If a repository with name does exist but is the wrong type, an error will be returned.
func ensureSnapshotRepo(ctx context.Context, client *elastic.Client, rType, name string, settings map[string]string) error {
	resp, err := client.SnapshotGetRepository(name).Repository(name).Do(context.Background())
	if err != nil && !elastic.IsNotFound(err) {
		// Unexpected error while checking if snapshot repository exists.
		logger.Error("error checking for existing snapshot repository", zap.Error(err))
		return err
	} else if repo, ok := resp[name]; elastic.IsNotFound(err) || !ok {
		// Snapshot repository doesn't exist. Create it.
		s := client.SnapshotCreateRepository(name).Type(rType)
		for k, v := range settings {
			s = s.Setting(k, v)
		}
		if _, err = s.Do(context.Background()); err != nil {
			logger.Error("error creating snapshot repository", zap.Error(err))
			return err
		}
	} else if ok && repo.Type != rType {
		// Snapshot repository exists, but is of the wrong type e.g. fs != s3.
		logger.Error(
			"snapshot repository exists, but is the wrong type",
			zap.String("want_type", rType),
			zap.String("got_type", repo.Type),
		)
		return err
	}
	return nil
}

// createSnapshot creates a new Elasticsearch snapshot for the given time.
//
// If now is more than one second greater or less than time.Now(), this func will panic.
func createSnapshot(ctx context.Context, client *elastic.Client, repoName string, now time.Time) error {
	// Sanity-check now: it should be pretty close to time.Now()
	if d := time.Since(now); -time.Second < d && d < time.Second {
		panic("now is not within one second of the current time")
	}
	snapshotName := now.Format(SnapshotFormat)
	logger.Info("creating snapshot", zap.String("snapshot", snapshotName))
	_, err := client.SnapshotCreate(repoName, snapshotName).WaitForCompletion(true).Do(ctx)
	if err != nil {
		logger.Error("error creating snapshot",
			zap.String("snapshot", snapshotName),
			zap.Error(err),
		)
		return err
	}
	return nil
}

// deleteOldSnapshots deletes Elaticsearch snapshots if they don't match schedule.
func deleteOldSnapshots(ctx context.Context, client *elastic.Client, repoName string, schedule SnapshotWindows) error {
	resp, err := client.SnapshotGet(repoName).Do(ctx)
	if err != nil {
		logger.Fatal("error getting existing snapshots", zap.Error(err))
		return err
	}
	for _, s := range resp.Snapshots {
		t, err := time.Parse(SnapshotFormat, s.Snapshot)
		if err != nil {
			logger.Fatal("error parsing time from snapshot name",
				zap.String("snapshot", s.Snapshot),
				zap.Error(err),
			)
			return err
		}
		if !schedule.Keep(t) {
			logger.Info("deleting snapshot", zap.String("snapshot", s.Snapshot))
			if _, err := client.SnapshotDelete(repoName, s.Snapshot).Do(ctx); err != nil {
				logger.Fatal("error deleting old snapshot",
					zap.String("snapshot", s.Snapshot),
					zap.Error(err),
				)
				return err
			}
		}
	}
	return nil
}
