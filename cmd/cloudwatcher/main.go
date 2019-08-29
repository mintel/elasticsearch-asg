package main

import (
	"context"
	"time"

	elastic "github.com/olivere/elastic/v7"  // Elasticsearch client
	"go.uber.org/zap"                        // Logging
	kingpin "gopkg.in/alecthomas/kingpin.v2" // Command line option parser

	// AWS clients and stuff.
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"

	esasg "github.com/mintel/elasticsearch-asg" // Complex Elasticsearch services
	"github.com/mintel/elasticsearch-asg/cmd"   // Common logging setup func
)

// Request retry count/timeouts.
const (
	awsMaxRetries = 3
	esRetryInit   = 150 * time.Millisecond
	esRetryMax    = 1200 * time.Millisecond
)

// defaultURL is the default Elasticsearch URL.
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

	logger := cmd.SetupLogging()
	defer func() {
		// Make sure any buffered logs get flushed before exiting successfully.
		// This should never happen because cloudwatcher should never exit successfully,
		// but just in case...
		// Subsequent calls to loger.Fatal() perform their own Sync().
		// See: https://github.com/uber-go/zap/blob/master/FAQ.md#why-include-dedicated-panic-and-fatal-log-levels
		// Do this inside a closure func so that the linter will stop complaining
		// about not checking the error output of Sync().
		_ = logger.Sync()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Make AWS clients.
	sess := session.Must(session.NewSession())
	awsConfig := aws.NewConfig().WithMaxRetries(awsMaxRetries)
	if region != nil && *region != "" {
		// If --region was set use that...
		awsConfig = awsConfig.WithRegion(*region)
	} else if region, _ := ec2metadata.New(sess).Region(); region != "" {
		// ... else try to get current region from EC2 Instance Metadata endpoint.
		awsConfig = awsConfig.WithRegion(region)
	}
	cwClient := cloudwatch.New(sess, awsConfig)
	ec2Client := ec2.New(sess, awsConfig)

	// Make Elasticsearch client.
	esClient, err := elastic.Dial(
		elastic.SetURL((*esURL).String()),
		elastic.SetRetrier(elastic.NewBackoffRetrier(elastic.NewExponentialBackoff(esRetryInit, esRetryMax))),
	)
	if err != nil {
		logger.Fatal("error creating Elasticsearch client", zap.Error(err))
	}

	esQuery := esasg.NewElasticsearchQueryService(esClient)


	var nodes map[string]*esasg.Node
	var vcpuCounts map[string]int
	for range time.NewTicker(*interval).C { // Each --interval...

		// Get info about all the nodes in Elasticsearch.
		nodes, err = esQuery.Nodes(ctx)
		if err != nil {
			logger.Error("error getting Elasticsearch nodes info", zap.Error(err))
			continue
		}

		// Get a count of vCPUs per instance. This is use when calculating Load %.
		instanceIDs := make([]string, 0, len(nodes))
		for id := range nodes {
			instanceIDs = append(instanceIDs, id)
		}
		vcpuCounts, err = GetInstanceVCPUCount(ec2Client, instanceIDs)
		if err != nil {
			logger.Error("error getting EC2 instances vCPU counts", zap.Error(err))
			continue
		}

		// Generate CloudWatch metric data points from nodes and vcpu counts.
		metricData := MakeCloudwatchData(nodes, vcpuCounts)
		for _, datum := range metricData {
			LogDatum(logger, datum)
		}
		if err = PushCloudwatchData(ctx, cwClient, metricData); err != nil {
			logger.Error("error pushing metrics to CloudWatch", zap.Error(err))
			continue
		}
	}
}
