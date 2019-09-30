# healthchecker

[![Docker Cloud Build Status](https://img.shields.io/docker/cloud/build/mintel/elasticsearch-healthchecker.svg)](https://hub.docker.com/r/mintel/elasticsearch-healthchecker)

Serve health (`/livez`) and readiness (`/readyz`) checks for an Elasticsearch node.

Healthy if:

- HEAD request to `/` succeeds.

Ready if:

- The node has joined a cluster.
- Only once at startup: the cluster state is green, or the cluster state is yellow but no shards are being initialized or relocated.

Checks are also served as [Prometheus gauges](https://prometheus.io/docs/concepts/metric_types/#gauge) at `/metricsz`.

## Usage

```sh
usage: healthchecker [<flags>]

Serve liveness and readiness checks for Elasticsearch.

Flags:
      --help                   Show context-sensitive help (also try --help-long and --help-man).
  -e, --elasticsearch.url=http://127.0.0.1:9200 ...
                               URL(s) of Elasticsearch.
      --log.level=INFO         Set logging level.
      --serve.port=8080        Port on which to expose health checks and Prometheus metrics.
      --serve.metrics="/metrics"
                               Path at which to serve Prometheus metrics.
      --serve.live="/livez"    Path at which to liveness healthcheck.
      --serve.ready="/readyz"  Path at which to serve Prometheus metrics.
```
