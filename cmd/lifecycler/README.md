# lifecycler

[![Docker Cloud Build Status](https://img.shields.io/docker/cloud/build/mintel/elasticsearch-lifecycler.svg)](https://hub.docker.com/r/mintel/elasticsearch-lifecycler)

Regulate AWS Autoscaling of Elasticsearch by delaying new autoscaling actions until cluster is stable.

## Why

Consider an Elasticsearch cluster running on an AWS Autoscaling Group, configured to scale in response to high CPU usage.
Once more nodes are added to the cluster, Elasticsearch will rebalance the shards to spread them evenly across the cluster.
This will cause high CPU usage, which will trigger more autoscaling, which will cause high CPU, which will....

AWS Autoscaling Groups provide [Lifecycle Hooks](https://docs.aws.amazon.com/autoscaling/ec2/userguide/lifecycle-hooks.html),
which allow setup or teardown operations when an autoscaled instance is launching or terminating.
The Autoscaling Group won't scale up/down again until the Lifecycle Hook event finishes i.e. its timeout expires.

Lifecycler consumes Lifecycle Hook events from an SQS queue, and:

1. If the instance is terminating, drains shards from the node and excludes if from master voting.
2. Delay the Lifecycle Hook event from timing out until the cluster reaches a green state.

This prevents the Autoscaling Group from getting into a feedback loop.

## Usage

```bash
usage: lifecycler [<flags>] <queue> [<url>]

Handle AWS Autoscaling Group Lifecycle hook events for Elasticsearch from
an SQS queue.

Flags:
      --help     Show context-sensitive help (also try --help-long and
                 --help-man).
  -v, --verbose  Show debug logging.

Args:
  <queue>  URL of SQS queue receiving lifecycle hook events.
  [<url>]  Elasticsearch URL. Default: http://localhost:9200
```
