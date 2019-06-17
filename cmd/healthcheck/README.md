# healthcheck

Serve health (`/live`) and readiness (`/ready`) checks for an Elasticsearch node.

Healthy if:

- HEAD request to `/` succeeds.

Ready if:

- The node has joined a cluster.
- Only once at startup: the cluster state is green, or the cluster state is yellow but no shards are being initialized or relocated.

Checks are also served as [Prometheus gauges](https://prometheus.io/docs/concepts/metric_types/#gauge) at `/metrics`.

## Usage

```bash
usage: healthcheck [<flags>] [<url>]

Handle AWS Autoscaling Group Lifecycle hook events for Elasticsearch from an SQS queue.

Flags:
      --help                     Show context-sensitive help (also try --help-long and --help-man).
  -v, --verbose                  Show debug logging.
      --port=9201                Port to serve healthchecks on.
      --namespace="elasticsearch"
                                 Namespace to use for Prometheus metrics.
      --once                     Execute checks once and exit with status code.
      --no-check-head            Disable HEAD check.
      --no-check-joined-cluster  Disable joined cluster check.
      --no-check-rolling-upgrade
                                 Disable rolling upgrade check.

Args:
  [<url>]  Elasticsearch URL. Default: http://localhost:9200
```
