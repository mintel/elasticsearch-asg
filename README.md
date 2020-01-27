# elasticsearch-asg

[![Build Status](https://travis-ci.org/mintel/elasticsearch-asg.svg?branch=master)](https://travis-ci.org/mintel/elasticsearch-asg)
[![GoDoc](https://godoc.org/github.com/mintel/elasticsearch-asg?status.svg)](https://godoc.org/github.com/mintel/elasticsearch-asg)
[![GitHub](https://img.shields.io/github/license/mintel/elasticsearch-asg.svg)](https://raw.githubusercontent.com/mintel/elasticsearch-asg/master/LICENSE)

A number of little applications I needed setting up Elasticsearch on AWS Autoscaling Groups.
These target Elasticsearch >= 7.0.

- [cloudwatcher] ([![Docker Cloud Build Status](https://img.shields.io/docker/cloud/build/mintel/elasticsearch-cloudwatcher.svg)](https://hub.docker.com/r/mintel/elasticsearch-cloudwatcher)) - Push metrics about an Elasticsearch cluster to AWS CloudWatch, mainly to inform [AWS Target Tracing Scaling Policies].

- [drainer] ([![Docker Cloud Build Status](https://img.shields.io/docker/cloud/build/mintel/elasticsearch-drainer.svg)](https://hub.docker.com/r/mintel/elasticsearch-drainer)) - Remove shards from Elasticsearch nodes on EC2 instances that are about to be terminated - either by an AWS AutoScaling Group downscaling or by Spot Instance interruption - by consuming [CloudWatch Events] from an [SQS Queue].

- [throttler] ([![Docker Cloud Build Status](https://img.shields.io/docker/cloud/build/mintel/elasticsearch-throttler.svg)](https://hub.docker.com/r/mintel/elasticsearch-throttler)) - Regulate an AWS AutoScaling Group running Elasticsearch by preventing new autoscaling actions until the cluster is stable (not red, no relocating shards, etc).

- [snapshooter] ([![Docker Cloud Build Status](https://img.shields.io/docker/cloud/build/mintel/elasticsearch-snapshooter.svg)](https://hub.docker.com/r/mintel/elasticsearch-snapshooter)) - Take snapshots of Elasticsearch cluster on a schedule, and clean up old ones with downsampling.

## Directory layout

- `/cmd` - A little bit of glue to run each application.
- `/pkg` - Packages that should theoretically be reusable in other projects.
- `/internal` - Code that isn't reusable in other projects.
  - `/internal/app` - The implementation of each application.
  - `/internal/pkg` - Packages used across multiple applications.

## How-To

### Start developing

You need a working [Go >= 1.13 environment](https://golang.org/doc/install).
Then clone this repository:

```sh
mkdir -p $GOPATH/src/github.com/mintel/
cd $GOPATH/src/github.com/mintel/
git clone git@github.com:mintel/elasticsearch-asg.git
cd elasticsearch-asg
```

### Run tests

```sh
go test ./...
```

### Build binaries

```sh
for app in cloudwatcher drainer snapshooter throttler; do
    go build "./cmd/$app"
done
```

### Build Docker containers

```sh
for app in cloudwatcher drainer snapshooter throttler; do
    docker build . -f "Dockerfile.$app" -t "mintel/elasticsearch-$app:latest"
done
```

### Debug

Install [Delve].

To debug one of the apps:
```sh
dlv debug ./cmd/cloudwatcher -- --elasticsearch.url=http://localhost:9200
```

To debug tests in the current directory:
```sh
dlv test
```

See the [Delve Getting Started] guide for more details.

### Make a new release

If you tag a commit with a semantic version, Docker Hub will build it as a separate tag. For example:

```sh
git tag -a v1.2.3 -m "Release v1.2.3"
git push --follow-tags
```

Will build Docker images like `mintel/elasticsearch-cloudwatcher:1.2.3`.

<!-- Links -->
[AWS Target Tracing Scaling Policies]: https://docs.aws.amazon.com/autoscaling/ec2/userguide/as-scaling-target-tracking.html
[cloudwatcher]: cmd/cloudwatcher
[drainer]: cmd/drainer
[throttler]: cmd/throttler
[snapshooter]: cmd/snapshooter
[Delve]: https://github.com/go-delve/delve
[Delve Getting Started]: https://github.com/go-delve/delve/blob/master/Documentation/cli/getting_started.md
[CloudWatch Events]: https://docs.aws.amazon.com/AmazonCloudWatch/latest/events/WhatIsCloudWatchEvents.html
[SQS Queue]: https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/welcome.html
<!-- /Links -->
