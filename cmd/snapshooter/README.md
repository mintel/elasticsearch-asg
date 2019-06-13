# elasticsearch-snapshooter

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
