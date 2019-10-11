package cloudwatcher

import (
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
// cloudwatcher App.
type Flags struct {
	// CloudWatch namespace to push metrics to.
	Namespace string

	// The interval at which cloudwatcher should poll
	// Elasticsearch for status information.
	PollInterval time.Duration

	*cmd.AWSFlags
	*cmd.ElasticsearchFlags
	*cmd.MonitoringFlags
}

// NewFlags returns a new Flags.
func NewFlags(app *kingpin.Application) *Flags {
	var f Flags

	app.Flag("namespace", "Name of the CloudWatch metrics namespace to use.").
		Short('n').
		Default("Elasticsearch").
		StringVar(&f.Namespace)

	app.Flag("interval", "The interval at which Elasticsearch should be polled for metric information.").
		Short('i').
		Default("1m").
		DurationVar(&f.PollInterval)

	f.AWSFlags = cmd.NewAWSFlags(app, _defaultAWSMaxRetries)
	f.ElasticsearchFlags = cmd.NewElasticsearchFlags(app, _defaultElasticsearchRetryInit, _defaultElasticsearchRetryMax)
	f.MonitoringFlags = cmd.NewMonitoringFlags(app, _defaultPort, _defaultLogLevel)

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
