# elasticsearch-asg

[![Build Status](https://travis-ci.org/mintel/elasticsearch-asg.svg?branch=master)](https://travis-ci.org/mintel/elasticsearch-asg)
[![GoDoc](https://godoc.org/github.com/mintel/elasticsearch-asg?status.svg)](https://godoc.org/github.com/mintel/elasticsearch-asg)
[![GitHub](https://img.shields.io/github/license/mintel/elasticsearch-asg.svg)](https://raw.githubusercontent.com/mintel/elasticsearch-asg/master/LICENSE)

A number of little tools I needed setting up Elasticsearch as AWS Autoscaling Groups.
These target Elasticsearch >= 7.0.

- [cloudwatcher](cmd/cloudwatcher) - Push metrics about Elasticsearch cluster to AWS CloudWatch, mainly for autoscaling.
- [healthcheck](cmd/healthcheck) - Provide health and readiness checks for Elasticsearch.
- [lifecycler](cmd/lifecycler) - Regulate AWS Autoscaling of Elasticsearch by delaying new autoscaling actions until cluster is stable.
- [snapshooter](cmd/snapshooter) - Take snapshots of Elasticsearch cluster on a schedule, and clean up old ones.
