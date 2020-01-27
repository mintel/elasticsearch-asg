# drainer

[![Docker Cloud Build Status](https://img.shields.io/docker/cloud/build/mintel/elasticsearch-drainer.svg)](https://hub.docker.com/r/mintel/elasticsearch-drainer)

## Usage

```sh
usage: drainer --queue=SQS_QUEUE_URL [<flags>]

Remove shards from Elasticsearch nodes on EC2 instances that are about to be terminated, either by an AWS AutoScaling Group downscaling or by Spot Instance interruption, by consuming CloudWatch Events from an SQS Queue. It assumes that Elasticsearch node names == EC2 instance ID.

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

## CloudWatch Events

Drainer can receive two kinds of events from CloudWatch Events:

- [EC2 Instance-terminate Lifecycle Action](https://docs.aws.amazon.com/autoscaling/ec2/userguide/cloud-watch-events.html#terminate-lifecycle-action).
- [Spot Instance interruption notices](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-interruptions.html#spot-instance-termination-notices).

Both event types should be sent into an SQS queue that drainer consumes from.
See also: https://docs.aws.amazon.com/AmazonCloudWatch/latest/events/resource-based-policies-cwe.html#sqs-permissions
