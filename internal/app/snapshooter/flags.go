package snapshooter

import (
	"errors"
	"time"

	kingpin "gopkg.in/alecthomas/kingpin.v2" // Command line flag parsing.

	"github.com/mintel/elasticsearch-asg/internal/pkg/cmd" // Common command line app tools.
	"github.com/mintel/elasticsearch-asg/pkg/retention"    // How long to keep backups.
)

const (
	defaultPort                   = 8080
	defaultLogLevel               = "INFO"
	defaultElasticsearchRetryInit = 150 * time.Millisecond
	defaultElasticsearchRetryMax  = 15 * time.Minute
)

// Flags holds command line flags for the
// snapshooter App.
type Flags struct {
	// Snapshot repository to use.
	Repository Repository

	// Frequency of snapshots.
	Config retention.Config

	// If true, clean up old snapshots.
	Delete bool

	// If true, print one cycle of snapshot
	// creation/deletion and exit without actually
	// performing any of the actions.
	DryRun bool

	*cmd.ElasticsearchFlags
	*cmd.MonitoringFlags
}

// NewFlags returns a new Flags.
func NewFlags(app *kingpin.Application) *Flags {
	var f Flags

	app.Flag("hourly", "Number of hourly snapshots to keep.").
		UintVar(&f.Config.Hourly)

	app.Flag("daily", "Number of daily snapshots to keep.").
		UintVar(&f.Config.Daily)

	app.Flag("weekly", "Number of weekly snapshots to keep.").
		UintVar(&f.Config.Weekly)

	app.Flag("monthly", "Number of monthly snapshots to keep.").
		UintVar(&f.Config.Monthly)

	app.Flag("yearly", "Number of yearly snapshots to keep.").
		UintVar(&f.Config.Yearly)

	app.Validate(func(app *kingpin.Application) error {
		if f.Config.MinInterval() == -1 {
			return errors.New("At least one of --hourly, --daily, --weekly, --month, --yearly must be set.")
		}
		return nil
	})

	app.Flag("repo.name", "The name of the snapshot repository to use.").
		Required().
		Short('r').
		StringVar(&f.Repository.Name)

	app.Flag("repo.type", "Ensure a snapshot repository with this type and --repo.name exists.").
		StringVar(&f.Repository.Type)

	app.Flag("repo.settings", "Settings to create snapshot repository with. See also: --repo.name and --repo.type.").
		StringMapVar(&f.Repository.Settings)

	app.Flag("delete", "Delete old snapshots. Not enabled by default for safety.").
		Short('d').
		BoolVar(&f.Delete)

	app.Flag("dry-run", "If set, print actions without taking them.").
		BoolVar(&f.DryRun)

	f.ElasticsearchFlags = cmd.NewElasticsearchFlags(app, defaultElasticsearchRetryInit, defaultElasticsearchRetryMax)
	f.MonitoringFlags = cmd.NewMonitoringFlags(app, defaultPort, defaultLogLevel)

	return &f
}
