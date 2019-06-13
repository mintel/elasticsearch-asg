# cloudwatcher

```bash
usage: cloudwatcher [<flags>] [<url>]

Push Elasticsearch metrics to AWS CloudWatch to run AWS Autoscaling
Groups.

Flags:
      --help           Show context-sensitive help (also try --help-long
                       and --help-man).
  -v, --verbose        Show debug logging.
      --interval=1m    Time between pushing metrics.
      --region=REGION  AWS Region.
      --namespace="Elasticsearch"
                       AWS CloudWatch metrics namespace.

Args:
  [<url>]  Elasticsearch URL. Default: http://localhost:9200
```
