package main

import (
	"context"
	"strconv"
	"time"

	elastic "github.com/olivere/elastic/v7"
	"go.uber.org/zap"
	kingpin "gopkg.in/alecthomas/kingpin.v2"

	esasg "github.com/mintel/elasticsearch-asg"
)

const (
	esRetryInit = 150 * time.Millisecond
	esRetryMax  = 1200 * time.Millisecond
)

// SnapshotFormat is the format for snapshot names (time.Time.Format())
const SnapshotFormat = "2006-01-02-15-04-05"

// Command line opts
var (
	esHost  = kingpin.Arg("url", "Elasticsearch URL. Default: http://localhost:9200").Default("http://localhost:9200").String()
	windows = kingpin.Flag("window", "Snapshot frequency + TTL. May be set multiple times. ISO 8601 Duration string format. Example: `--window P1M=PT1H` == keep hourly snapshots for 1 month.").Default(
		"P1M=PT1H",
		"P3M=P1W",
		"P3Y=P1M",
	).StringMap()
	repoName     = kingpin.Flag("repo", "Name of the snapshot repository.").Default("backups").String()
	repoType     = kingpin.Flag("type", "If set, create a repository of this type before creating snapshots. See also: '--settings'").String()
	repoSettings = kingpin.Flag("settings", "Use these settings creating the snapshot repository. May be set multiple times. Example: `--type=s3 --settings bucket=my_bucket`").StringMap()
)

func main() {
	kingpin.CommandLine.Help = "Create and clean up Elasticsearch snapshots on a schedule."
	kingpin.Parse()

	repoName := *repoName
	logger := esasg.SetupLogging().With(zap.String("snapshot_repository", repoName))
	defer func() {
		err := logger.Sync()
		if err != nil {
			panic(err)
		}
	}()

	snapshotWindows := make(SnapshotWindows, 0)
	for keepFor, every := range *windows {
		w, err := NewSnapshotWindow(every, keepFor)
		if err != nil {
			logger.Fatal("error parsing snapshot window",
				zap.String("keepFor", keepFor),
				zap.String("every", every),
				zap.Error(err),
			)
		}
		snapshotWindows = append(snapshotWindows, w)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := elastic.DialContext(
		ctx,
		elastic.SetURL(*esHost),
		elastic.SetRetrier(elastic.NewBackoffRetrier(elastic.NewExponentialBackoff(esRetryInit, esRetryMax))),
	)
	if err != nil {
		logger.Fatal("error creating Elasticsearch client", zap.Error(err))
	}

	if repoType != nil && *repoType != "" {
		resp, err := client.SnapshotGetRepository(repoName).Do(ctx)
		if _, ok := resp[repoName]; err != nil && !elastic.IsNotFound(err) {
			logger.Fatal("error checking for existing snapshot repository", zap.Error(err))
		} else if elastic.IsNotFound(err) || !ok {
			// Repo doesn't exist. Create it.
			s := client.SnapshotCreateRepository(repoName).Type(*repoType)
			for k, v := range *repoSettings {
				s = s.Setting(k, v)
			}
			if _, err = s.Do(ctx); err != nil {
				logger.Fatal("error creating snapshot repository", zap.Error(err))
			}
		}
	}

	nextSnapshot := snapshotWindows.Next()
	startSnapshot := time.After(time.Until(nextSnapshot))

	for {
		logger.Debug("waiting till next snapshot", zap.Time("time", nextSnapshot))
		<-startSnapshot

		snapshotName := nextSnapshot.Format(SnapshotFormat)

		// Schedule the next snapshot
		nextSnapshot = snapshotWindows.Next()
		startSnapshot = time.After(time.Until(nextSnapshot))

		// Create new snapshot
		logger.Info("creating snapshot " + snapshotName)
		timeout := strconv.FormatInt(int64(time.Until(nextSnapshot).Minutes()), 10) + "m"
		_, err := client.SnapshotCreate(repoName, snapshotName).WaitForCompletion(true).MasterTimeout(timeout).Do(ctx)
		if err != nil {
			logger.Fatal("error creating snapshot",
				zap.String("snapshot_name", snapshotName),
				zap.Error(err),
			)
		}

		// Clean up old snapshots
		resp, err := client.SnapshotGet(repoName).Do(ctx)
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
			if !snapshotWindows.Keep(t) {
				_, err := client.SnapshotDelete(repoName, s.Snapshot).Do(ctx)
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
