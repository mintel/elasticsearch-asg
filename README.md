# elasticsearch-asg

A number of little tools I needed setting up Elasticsearch as AWS Autoscaling Groups.

- [cloudwatcher](cmd/cloudwatcher) - Push metrics about Elasticsearch cluster to AWS CloudWatch, mainly for autoscaling.
- [healthcheck](cmd/healthcheck) - Provide health and readiness checks for Elasticsearch.
- [lifecylcer](cmd/lifecylcer) - Regulate AWS Autoscaling of Elasticsearch by delaying new autoscaling actions until cluster is stable.
- [snapshooter](cmd/snapshooter) - Take snapshots of Elasticsearch cluster on a schedule, and clean up old ones.
