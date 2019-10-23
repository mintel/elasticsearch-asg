# throttler

[![Docker Cloud Build Status](https://img.shields.io/docker/cloud/build/mintel/elasticsearch-asg-throttler.svg)](https://hub.docker.com/r/mintel/elasticsearch-asg-throttler)

## Usage

```sh
usage: throttler --group=AUTOSCALING_GROUP_NAME [<flags>]

Regulate an AWS AutoScaling Group running Elasticsearch by preventing new autoscaling actions until the cluster is stable (not red, no relocating shards, etc).

Flags:
      --help                    Show context-sensitive help (also try --help-long and --help-man).
  -g, --group=AUTOSCALING_GROUP_NAME ...
                                Name of AWS AutoScaling Group to enable/disable scaling on.
  -i, --interval=1m             The interval at which Elasticsearch should be polled for status information.
      --dry-run                 Log actions without actually taking them.
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
