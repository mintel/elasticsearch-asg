package drainer

import (
	"net/url"
	"time"

	kingpin "gopkg.in/alecthomas/kingpin.v2" // Command line flag parsing.

	"github.com/mintel/elasticsearch-asg/internal/pkg/cmd" // Common command line app tools.
)

const (
	_defaultPort                   = 8080
	_defaultLogLevel               = "INFO"
	_defaultAWSMaxRetries          = 5
	_defaultElasticsearchRetryInit = 150 * time.Millisecond
	_defaultElasticsearchRetryMax  = 15 * time.Minute
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
	*cmd.LoggingFlags
	*cmd.ServerFlags
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

	f.AWSFlags = cmd.NewAWSFlags(app, _defaultAWSMaxRetries)
	f.ElasticsearchFlags = cmd.NewElasticsearchFlags(app, _defaultElasticsearchRetryInit, _defaultElasticsearchRetryMax)
	f.LoggingFlags = cmd.NewLoggingFlags(app, _defaultLogLevel)
	f.ServerFlags = cmd.NewServerFlags(app, _defaultPort)

	return &f
}
