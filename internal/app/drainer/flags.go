package drainer

import (
	"net/url"
	"time"

	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/mintel/elasticsearch-asg/internal/pkg/cmd"
)

const (
	defaultPort                   = 8080
	defaultLogLevel               = "INFO"
	defaultAWSMaxRetries          = 5
	defaultElasticsearchRetryInit = 150 * time.Millisecond
	defaultElasticsearchRetryMax  = 1200 * time.Millisecond
)

// Flags holds command line flags for the
// drainer App.
type Flags struct {
	// The URL of an SQS queue which is configured to receive
	// CloudWatch events from Spot Instance Interruptions and
	// AutoScaling termination events from the Elasticsearch
	// AutoScaling Groups.
	Queue *url.URL

	// The interval at which drainer should poll
	// Elasticsearch for status information.
	PollInterval time.Duration

	*cmd.AWSFlags
	*cmd.ElasticsearchFlags
	*cmd.MonitoringFlags
}

// NewFlags returns a new Flags.
func NewFlags(app *kingpin.Application) *Flags {
	var f Flags

	app.Flag("queue", "URL of the SQS queue to receive CloudWatch events from.").
		Short('q').
		Required().
		PlaceHolder("SQS_QUEUE_URL").
		URLVar(&f.Queue)

	app.Flag("interval", "The interval at which Elasticsearch should be polled for metric information.").
		Short('i').
		Default("1m").
		DurationVar(&f.PollInterval)

	f.AWSFlags = cmd.NewAWSFlags(app, defaultAWSMaxRetries)
	f.ElasticsearchFlags = cmd.NewElasticsearchFlags(app, defaultElasticsearchRetryInit, defaultElasticsearchRetryMax)
	f.MonitoringFlags = cmd.NewMonitoringFlags(app, defaultPort, defaultLogLevel)

	return &f
}
