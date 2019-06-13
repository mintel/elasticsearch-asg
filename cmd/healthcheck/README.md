# healthcheck

```bash
usage: healthcheck [<flags>] [<url>]

Handle AWS Autoscaling Group Lifecycle hook events for Elasticsearch from an SQS queue.

Flags:
      --help            Show context-sensitive help (also try --help-long and --help-man).
  -v, --verbose         Show debug logging.
      --endpoint=:9201  Endpoint to serve healthchecks at.

Args:
  [<url>]  Elasticsearch URL. Default: http://localhost:9200
```
