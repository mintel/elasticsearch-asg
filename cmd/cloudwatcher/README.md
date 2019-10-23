# cloudwatcher

[![Docker Cloud Build Status](https://img.shields.io/docker/cloud/build/mintel/elasticsearch-cloudwatcher.svg)](https://hub.docker.com/r/mintel/elasticsearch-cloudwatcher)

## Usage

```sh
usage: cloudwatcher [<flags>]

Cloudwatcher pushes metrics about an Elasticsearch cluster to AWS CloudWatch, mainly to inform AWS Target Tracing Scaling Policies.

The metrics include:

  - File system utilization (data nodes only)
  - JVM heap utilization (both in total, and per-memory pool)
  - JVM garbage collection stats

The metrics are both in total, and broken out by node role (master, data, etc...).

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
      --serve.port=8080         Port on which to expose healthchecks and Prometheus metrics.
      --serve.metrics="/metrics"
                                Path at which to serve Prometheus metrics.
      --serve.live="/livez"     Path at which to serve liveness healthcheck.
      --serve.ready="/readyz"   Path at which to serve readiness healthcheck.
```

<!-- Links -->
[AWS Target Tracing Scaling Policies]: https://docs.aws.amazon.com/autoscaling/ec2/userguide/as-scaling-target-tracking.html
<!-- /Links -->
