# elasticsearch-asg

[![Build Status](https://travis-ci.org/mintel/elasticsearch-asg.svg?branch=master)](https://travis-ci.org/mintel/elasticsearch-asg)
[![GoDoc](https://godoc.org/github.com/mintel/elasticsearch-asg?status.svg)](https://godoc.org/github.com/mintel/elasticsearch-asg)
[![GitHub](https://img.shields.io/github/license/mintel/elasticsearch-asg.svg)](https://raw.githubusercontent.com/mintel/elasticsearch-asg/master/LICENSE)

A number of little applications I needed setting up Elasticsearch as AWS Autoscaling Groups.
These target Elasticsearch >= 7.0.

- [cloudwatcher] ([![Docker Cloud Build Status](https://img.shields.io/docker/cloud/build/mintel/elasticsearch-cloudwatcher.svg)](https://hub.docker.com/r/mintel/elasticsearch-cloudwatcher)) - Push metrics about Elasticsearch cluster to AWS CloudWatch, mainly to inform autoscaling.

- [healthchecker] ([![Docker Cloud Build Status](https://img.shields.io/docker/cloud/build/mintel/elasticsearch-healthchecker.svg)](https://hub.docker.com/r/mintel/elasticsearch-healthchecker)) - Provide health and readiness checks for Elasticsearch.

- [drainer] ([![Docker Cloud Build Status](https://img.shields.io/docker/cloud/build/mintel/elasticsearch-drainer.svg)](https://hub.docker.com/r/mintel/elasticsearch-drainer)) - Remove shards from Elasticsearch nodes on EC2 instances that are about to be terminated.

- [throttler] ([![Docker Cloud Build Status](https://img.shields.io/docker/cloud/build/mintel/elasticsearch-asg-throttler.svg)](https://hub.docker.com/r/mintel/elasticsearch-asg-throttler)) - Regulate AWS autoscaling of Elasticsearch by delaying new autoscaling actions until cluster is stable.

- [snapshooter] ([![Docker Cloud Build Status](https://img.shields.io/docker/cloud/build/mintel/elasticsearch-snapshooter.svg)](https://hub.docker.com/r/mintel/elasticsearch-snapshooter)) - Take snapshots of Elasticsearch cluster on a schedule, and clean up old ones with downsampling.

## To start developing

### You need a working [Go >= 1.13 environment](https://golang.org/doc/install).

```sh
mkdir -p $GOPATH/src/github.com/mintel/
cd $GOPATH/src/github.com/mintel/
git clone git@github.com:mintel/elasticsearch-asg.git
cd elasticsearch-asg
go test ./...
```

## Directory layout

- `/cmd` - A little bit of glue to run each application.
- `/pkg` - Packages that should theoretically be reusable in other projects.
- `/internal` - Code that isn't reusable in other projects.
  - `/internal/app` - The implementation of each application.
  - `/internal/pkg` - Packages used across multiple applications.

<!-- Links -->

[cloudwatcher]: cmd/cloudwatcher
[healthchecker]: cmd/healthchecker
[drainer]: cmd/drainer
[throttler]: cmd/throttler
[snapshooter]: cmd/snapshooter
