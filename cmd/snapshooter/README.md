# elasticsearch-snapshooter

Create and clean up Elasticsearch snapshots on a schedule.

## Example

```
$ ./snapshooter \
    --repo=backups --type=s3 --settings bucket=my_bucket \
    --window P1M=PT1H \
    --window P3M=P1W \
    --window P3Y=P1M \
```

Create a S3 snapshot repository named "backups" (if one doesn't already exist).

Then create hourly snapshots that are kept for one month, weekly snapshots that are kept for 3 months,
and monthly snapshots that are kept for 3 years.

## Usage

```bash
usage: snapshooter [<flags>] [<url>]

Create and clean up Elasticsearch snapshots on a schedule.

Flags:
      --help                    Show context-sensitive help (also try
                                --help-long and --help-man).
  -v, --verbose                 Show debug logging.
      --window=P1M=PT1H... ...  Snapshot frequency + TTL. May be set
                                multiple times. ISO 8601 Duration string
                                format. Example: `--window P1M=PT1H` ==
                                keep hourly snapshots for 1 month.
      --repo="backups"          Name of the snapshot repository.
      --type=TYPE               If set, create a repository of this type
                                before creating snapshots. See also:
                                '--settings'
      --settings=SETTINGS ...   Use these settings creating the snapshot
                                repository. May be set multiple times.
                                Example: `--type=s3 --settings
                                bucket=my_bucket`

Args:
  [<url>]  Elasticsearch URL. Default: http://localhost:9200
```
