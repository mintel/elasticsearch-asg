# cloudwatcher

[![Docker Cloud Build Status](https://img.shields.io/docker/cloud/build/mintel/elasticsearch-cloudwatcher.svg)](https://hub.docker.com/r/mintel/elasticsearch-cloudwatcher)

Cloudwatcher pushes metrics about an Elasticsearch cluster to AWS CloudWatch,
mainly to run AWS Autoscaling Groups. The metrics include:

- File system utilization (data nodes only)
- JVM heap utilization (both in total, and per-memory pool)
- JVM garbage collection stats

The metrics are both in total, and broken out by node role (master, data, etc...).

## Usage

```sh
usage: cloudwatcher [<flags>]

Push Elasticsearch metrics to AWS CloudWatch, specifically to run AWS Autoscaling Groups for Elasticsearch.

Flags:
      --help                    Show context-sensitive help (also try --help-long and --help-man).
  -n, --namespace="Elasticsearch"
                                Name of the CloudWatch metrics namespace to use.
  -i, --interval=1m             The interval at which Elasticsearch should be polled for metric information.
      --aws.region=REGION_NAME  Name of AWS region to use.
      --aws.profile=PROFILE_NAME
                                Name of AWS credentials profile to use.
  -e, --elasticsearch.url=http://127.0.0.1:9200 ...
                                URL(s) of Elasticsearch.
      --log.level=INFO          Set logging level.
      --serve.port=8080         Port on which to expose health checks and Prometheus metrics.
      --serve.metrics="/metrics"
                                Path at which to serve Prometheus metrics.
      --serve.live="/livez"     Path at which to liveness healthcheck.
      --serve.ready="/readyz"   Path at which to serve Prometheus metrics.
```
