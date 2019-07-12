package main

import (
	"context"
	"time"

	elastic "github.com/olivere/elastic/v7"
	"go.uber.org/zap"
	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"

	esasg "github.com/mintel/elasticsearch-asg"
)

const (
	awsMaxRetries = 3
	esRetryInit   = 150 * time.Millisecond
	esRetryMax    = 1200 * time.Millisecond
)

const defaultURL = "http://localhost:9200"

// Command line opts
var (
	esURL     = kingpin.Arg("url", "Elasticsearch URL. Default: "+defaultURL).Default(defaultURL).URL()
	interval  = kingpin.Flag("interval", "Time between pushing metrics.").Default("1m").Duration()
	region    = kingpin.Flag("region", "AWS Region.").String()
	namespace = kingpin.Flag("namespace", "AWS CloudWatch metrics namespace.").Default("Elasticsearch").String()
)

func main() {
	kingpin.CommandLine.Help = "Push Elasticsearch metrics to AWS CloudWatch to run AWS Autoscaling Groups."
	kingpin.Parse()

	logger := esasg.SetupLogging()
	defer func() {
		err := logger.Sync()
		if err != nil {
			panic(err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sess := session.Must(session.NewSession())
	awsConfig := aws.NewConfig().WithMaxRetries(awsMaxRetries)
	if region != nil && *region != "" {
		awsConfig = awsConfig.WithRegion(*region)
	} else if region, _ := ec2metadata.New(sess).Region(); region != "" {
		awsConfig = awsConfig.WithRegion(region)
	}
	cwSvc := cloudwatch.New(sess, awsConfig)
	ec2Svc := ec2.New(sess, awsConfig)

	esClient, err := elastic.DialContext(
		ctx,
		elastic.SetURL((*esURL).String()),
		elastic.SetRetrier(elastic.NewBackoffRetrier(elastic.NewExponentialBackoff(esRetryInit, esRetryMax))),
	)
	if err != nil {
		logger.Fatal("error creating Elasticsearch client", zap.Error(err))
	}

	esQuery := esasg.NewElasticsearchQueryService(esClient)

	for range time.NewTicker(*interval).C {
		go func() {
			nodes, err := esQuery.Nodes(ctx)
			if err != nil {
				logger.Fatal("error getting Elasticsearch nodes info", zap.Error(err))
			}

			instanceIDs := make([]string, 0, len(nodes))
			for id := range nodes {
				instanceIDs = append(instanceIDs, id)
			}
			vcpuCounts, err := GetInstanceVCPUCount(ec2Svc, instanceIDs)
			if err != nil {
				logger.Fatal("error getting EC2 instances vCPU counts", zap.Error(err))
			}

			metricData := MakeCloudwatchData(nodes, vcpuCounts)
			for _, datum := range metricData {
				LogDatum(logger, datum)
			}

			// PutMetricData() sends a max of 40960 bytes.
			// Have to batch our requests.
			batchSize := 20 // This is probably small enough
			for i := 0; i < len(metricData); i += batchSize {
				j := i + batchSize
				if j > len(metricData) {
					j = len(metricData)
				}
				batch := metricData[i:j]
				_, err := cwSvc.PutMetricDataWithContext(ctx, &cloudwatch.PutMetricDataInput{
					Namespace:  namespace,
					MetricData: batch,
				})
				if err != nil {
					logger.Fatal("error pushing CloudWatch metrics", zap.Error(err))
				}
				logger.Info("pushed metrics to CloudWatch", zap.Int("count", len(batch)))
			}
		}()
	}
}

// LogDatum logs a CloudWatch data point at debug level.
func LogDatum(logger *zap.Logger, datum *cloudwatch.MetricDatum) {
	logger = logger.With(zap.String("name", *datum.MetricName), zap.Float64("value", *datum.Value), zap.Namespace("dimensions"))
	for _, d := range datum.Dimensions {
		logger = logger.With(zap.String(*d.Name, *d.Value))
	}
	logger.Debug("metric datum")
}
