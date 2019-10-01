package healthchecker

import (
	"time"

	kingpin "gopkg.in/alecthomas/kingpin.v2" // Command line flag parsing.

	"github.com/mintel/elasticsearch-asg/internal/pkg/cmd" // Common command line app tools.
)

const (
	defaultPort                   = 8080
	defaultLogLevel               = "INFO"
	defaultElasticsearchRetryInit = 150 * time.Millisecond
	defaultElasticsearchRetryMax  = 1200 * time.Millisecond
)

// Flags holds command line flags for the
// healthcheck App.
type Flags struct {
	// If true, check health once and exit with a status code.
	Once bool

	// Allow various checks to be disabled.
	DisableCheckHead           bool
	DisableCheckJoined         bool
	DisableCheckRollingUpgrade bool

	*cmd.ElasticsearchFlags
	*cmd.MonitoringFlags
}

// NewFlags returns a new Flags.
func NewFlags(app *kingpin.Application) *Flags {
	var f Flags

	app.Flag("once", "If true, check health once and exit with a status code.").
		BoolVar(&f.Once)

	app.Flag("no-check-head", "Disable HEAD check.").
		BoolVar(&f.DisableCheckHead)

	app.Flag("no-check-joined-cluster", "Disable joined cluster check.").
		BoolVar(&f.DisableCheckJoined)

	app.Flag("no-check-rolling-upgrade", "Disable rolling upgrade check.").
		BoolVar(&f.DisableCheckRollingUpgrade)

	f.ElasticsearchFlags = cmd.NewElasticsearchFlags(app, defaultElasticsearchRetryInit, defaultElasticsearchRetryMax)
	f.MonitoringFlags = cmd.NewMonitoringFlags(app, defaultPort, defaultLogLevel)

	return &f
}
