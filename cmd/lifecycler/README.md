# lifecycler

```bash
usage: lifecycler [<flags>] <queue> [<url>]

Handle AWS Autoscaling Group Lifecycle hook events for Elasticsearch from
an SQS queue.

Flags:
      --help     Show context-sensitive help (also try --help-long and
                 --help-man).
  -v, --verbose  Show debug logging.

Args:
  <queue>  URL of SQS queue.
  [<url>]  Elasticsearch URL. Default: http://localhost:9200
```
