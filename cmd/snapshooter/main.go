package main

import (
	"context"
	"strconv"
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
	esURL   = kingpin.Arg("url", "Elasticsearch URL. Default: "+defaultURL).Default(defaultURL).URL()
	windows = kingpin.Flag("window", "Snapshot frequency + TTL. May be set multiple times. ISO 8601 Duration string format. Example: `--window P1M=PT1H` == keep hourly snapshots for 1 month.").Default(
		"P1M=PT1H",
		"P3M=P1W",
		"P3Y=P1M",
	).StringMap()
	delete       = kingpin.Flag("delete", "If set, clean up old snapshots. This is false by default for safety's sake.").Short('d').Bool()
	repoName     = kingpin.Flag("repo", "Name of the snapshot repository.").Default("backups").String()
	repoType     = kingpin.Flag("type", "If set, create a repository of this type before creating snapshots. See also: '--settings'").String()
	repoSettings = kingpin.Flag("settings", "Use these settings creating the snapshot repository. May be set multiple times. Example: `--type=s3 --settings bucket=my_bucket`").StringMap()
)

func main() {
	kingpin.CommandLine.Help = "Create and clean up Elasticsearch snapshots on a schedule."
	kingpin.Parse()

	// Deference global repoName flag pointer to local variable.
	repoName := *repoName

	// Set up logger.
	logger := cmd.SetupLogging().With(zap.String("snapshot_repository", repoName))
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

	// Craete Elasticsearch client.
	client, err := elastic.Dial(
		elastic.SetURL((*esURL).String()),
		elastic.SetRetrier(elastic.NewBackoffRetrier(elastic.NewExponentialBackoff(esRetryInit, esRetryMax))),
	)
	if err != nil {
		logger.Fatal("error creating Elasticsearch client", zap.Error(err))
	}

	// If --type/--settings flags are set, create the snapshot repository if it doesn't exist.
	if repoType != nil && *repoType != "" {
		resp, err := client.SnapshotGetRepository(repoName).Repository(repoName).Do(context.Background())
		if err != nil && !elastic.IsNotFound(err) {
			// Unexpected error while checking if snapshot repository exists.
			logger.Fatal("error checking for existing snapshot repository", zap.Error(err))
		} else if repo, ok := resp[repoName]; elastic.IsNotFound(err) || !ok {
			// Snapshot repository doesn't exist. Create it.
			s := client.SnapshotCreateRepository(repoName).Type(*repoType)
			for k, v := range *repoSettings {
				s = s.Setting(k, v)
			}
			if _, err = s.Do(context.Background()); err != nil {
				logger.Fatal("error creating snapshot repository", zap.Error(err))
			}
		} else if ok && repo.Type != *repoType {
			// Snapshot repository exists, but is of the wrong type e.g. fs != s3.
			logger.Fatal(
				"snapshot repository exists, but is the wrong type",
				zap.String("want_type", *repoType),
				zap.String("got_type", repo.Type),
			)
		}
	}

	nextSnapshot := snapshotSchedule.Next()               // Time at which the next snapshot should occur.
	startSnapshot := time.After(time.Until(nextSnapshot)) // Timer that goes off at nextSnapshot.

	for {
		// Wait for startSnapshot timer to go off.
		logger.Debug("waiting till next snapshot", zap.Time("time", nextSnapshot))
		<-startSnapshot

		// Format the name of the new snapshot based of the scheduled time.
		snapshotName := nextSnapshot.Format(SnapshotFormat)

		// Reset the timer for the next snapshot.
		nextSnapshot = snapshotSchedule.Next()
		startSnapshot = time.After(time.Until(nextSnapshot))

		// Create the new snapshot, waiting for it to complete.
		logger.Info("creating snapshot " + snapshotName)
		timeout := strconv.FormatInt(int64(time.Until(nextSnapshot).Minutes()), 10) + "m"
		_, err := client.SnapshotCreate(repoName, snapshotName).
			MasterTimeout(timeout).
			WaitForCompletion(true).
			Do(context.Background())
		if err != nil {
			logger.Fatal("error creating snapshot",
				zap.String("snapshot_name", snapshotName),
				zap.Error(err),
			)
		}

		// If the --delete flag isn't set, don't clean up old snapshots.
		if !*delete {
			continue
		}

		// Clean up old snapshots.
		resp, err := client.SnapshotGet(repoName).Do(context.Background())
		if err != nil {
			logger.Fatal("error getting existing snapshots", zap.Error(err))
		}
		for _, s := range resp.Snapshots {
			t, err := time.Parse(SnapshotFormat, s.Snapshot)
			if err != nil {
				logger.Fatal("error parsing time from snapshot name",
					zap.String("snapshot_name", s.Snapshot),
					zap.Error(err),
				)
			}
			if !snapshotSchedule.Keep(t) {
				_, err := client.SnapshotDelete(repoName, s.Snapshot).Do(context.Background())
				if err != nil {
					logger.Fatal("error deleting old snapshot",
						zap.String("snapshot_name", s.Snapshot),
						zap.Error(err),
					)
				}
			}
		}
	}
}
