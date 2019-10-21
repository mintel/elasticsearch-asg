# snapshooter

[![Docker Cloud Build Status](https://img.shields.io/docker/cloud/build/mintel/elasticsearch-snapshooter.svg)](https://hub.docker.com/r/mintel/elasticsearch-snapshooter)

Create and clean up Elasticsearch snapshots on a schedule.

## Example

```
$ ./snapshooter \
    --repo=backups --type=s3 --settings bucket=my_bucket \
    --window P1M=PT1H \
    --window P3M=P1W \
    --window P3Y=P1M \
    --delete
```

Create a S3 snapshot repository named "backups" (if one doesn't already exist).

Then create hourly snapshots that are kept for one month, weekly snapshots that are kept for 3 months,
and monthly snapshots that are kept for 3 years. Delete old snapshots.

## Usage

```sh
usage: snapshooter --repo.name=REPO.NAME [<flags>]

Create period Elasticsearch snapshots, and delete old ones with downsampling.

Flags:
      --help                   Show context-sensitive help (also try --help-long and --help-man).
      --hourly=HOURLY          Number of hourly snapshots to keep.
      --daily=DAILY            Number of daily snapshots to keep.
      --weekly=WEEKLY          Number of weekly snapshots to keep.
      --monthly=MONTHLY        Number of monthly snapshots to keep.
      --yearly=YEARLY          Number of yearly snapshots to keep.
  -r, --repo.name=REPO.NAME    The name of the snapshot repository to use.
      --repo.type=REPO.TYPE    Ensure a snapshot repository with this type and --repo.name exists.
      --repo.settings=REPO.SETTINGS ...
                               Settings to create snapshot repository with. See also: --repo.name and --repo.type.
  -d, --delete                 Delete old snapshots. Not enabled by default for safety.
      --dry-run                If set, print actions without taking them.
  -e, --elasticsearch.url=http://127.0.0.1:9200 ...
                               URL(s) of Elasticsearch.
      --log.level=INFO         Set logging level.
      --serve.port=8080        Port on which to expose healthchecks and Prometheus metrics.
      --serve.metrics="/metrics"
                               Path at which to serve Prometheus metrics.
      --serve.live="/livez"    Path at which to serve liveness healthcheck.
      --serve.ready="/readyz"  Path at which to serve readiness healthcheck.
```
