# drainer

[![Docker Cloud Build Status](https://img.shields.io/docker/cloud/build/mintel/elasticsearch-drainer.svg)](https://hub.docker.com/r/mintel/elasticsearch-drainer)

Remove shards from Elasticsearch nodes on EC2 instances that are about to be terminated.

## Usage

```sh
usage: drainer --queue=SQS_QUEUE_URL [<flags>]

Remove shards from Elasticsearch nodes on EC2 instances that are about to be terminated.

Flags:
      --help                    Show context-sensitive help (also try --help-long and --help-man).
  -q, --queue=SQS_QUEUE_URL     URL of the SQS queue to receive CloudWatch events from.
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