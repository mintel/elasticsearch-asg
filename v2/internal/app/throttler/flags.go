package throttler

import (
	"time"

	kingpin "gopkg.in/alecthomas/kingpin.v2" // Command line flag parsing.

	"github.com/mintel/elasticsearch-asg/v2/internal/pkg/cmd" // Common command line app tools.
)

const (
	_defaultPort                   = 8080
	_defaultLogLevel               = "INFO"
	_defaultAWSMaxRetries          = 5
	_defaultElasticsearchRetryInit = 150 * time.Millisecond
	_defaultElasticsearchRetryMax  = 15 * time.Minute
)

// Flags holds command line flags for the
// throttler App.
type Flags struct {
	// Names of an AutoScaling Groups to enable/disable
	// scaling on based on Elasticsearch status.
	AutoScalingGroupNames []string

	// The interval at which throttler should poll
	// Elasticsearch for status information.
	PollInterval time.Duration

	// If true, log actions without actually taking them.
	DryRun bool

	*cmd.AWSFlags
	*cmd.ElasticsearchFlags
	*cmd.LoggingFlags
	*cmd.ServerFlags
}

// NewFlags returns a new Flags.
func NewFlags(app *kingpin.Application) *Flags {
	var f Flags

	app.Flag("group", "Name of AWS AutoScaling Group to enable/disable scaling on.").
		Short('g').
		Required().
		PlaceHolder("AUTOSCALING_GROUP_NAME").
		StringsVar(&f.AutoScalingGroupNames)

	app.Flag("interval", "The interval at which Elasticsearch should be polled for status information.").
		Short('i').
		Default("1m").
		DurationVar(&f.PollInterval)

	app.Flag("dry-run", "Log actions without actually taking them.").
		BoolVar(&f.DryRun)

	f.AWSFlags = cmd.NewAWSFlags(app, _defaultAWSMaxRetries)
	f.ElasticsearchFlags = cmd.NewElasticsearchFlags(app, _defaultElasticsearchRetryInit, _defaultElasticsearchRetryMax)
	f.LoggingFlags = cmd.NewLoggingFlags(app, _defaultLogLevel)
	f.ServerFlags = cmd.NewServerFlags(app, _defaultPort)

	return &f
}

// Tick returns a channel on which
func (f *Flags) Tick() <-chan time.Time {
	c := make(chan time.Time)
	go func(c chan<- time.Time) {
		// Send one tick right away.
		c <- time.Now()

		for t := range time.Tick(f.PollInterval) {
			// Mimic the non-blocking behavior of time.Ticker.
			select {
			case c <- t:
			default:
			}
		}
	}(c)
	return c
}
