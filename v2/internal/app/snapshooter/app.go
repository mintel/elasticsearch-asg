package snapshooter

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"time"

	elastic "github.com/olivere/elastic/v7"          // Elasticsearch client.
	"github.com/prometheus/client_golang/prometheus" // Prometheus metrics.
	"go.uber.org/zap"                                // Logging.
	kingpin "gopkg.in/alecthomas/kingpin.v2"         // Command line flag parsing.

	"github.com/mintel/elasticsearch-asg/v2/internal/pkg/cmd"     // Common command line app tools.
	"github.com/mintel/elasticsearch-asg/v2/internal/pkg/metrics" // Prometheus metrics tools.
	"github.com/mintel/elasticsearch-asg/v2/pkg/retention"        // How long to keep backups.
)

const (
	Name  = "snapshooter"
	Usage = "Take snapshots of Elasticsearch cluster on a schedule, and clean up old ones with downsampling."
)

// App holds application state.
type App struct {
	*kingpin.Application

	flags  *Flags           // Command line flags
	health *Healthchecks    // healthchecks HTTP handler
	inst   *Instrumentation // App-specific Prometheus metrics

	// API clients.
	clients struct {
		ElasticsearchHTTP *http.Client
		Elasticsearch     *elastic.Client
	}
}

// NewApp returns a new App.
func NewApp(r prometheus.Registerer) (*App, error) {
	namespace := cmd.BuildPromFQName("", Name)

	app := &App{
		Application: kingpin.New(filepath.Base(os.Args[0]), Usage),
	}
	app.flags = NewFlags(app.Application)
	app.inst = NewInstrumentation(namespace)
	if err := r.Register(app.inst); err != nil {
		return nil, err
	}

	// Add post-flag-parsing actions.
	// These should only return an error if that error
	// is related to user input in some way, since kingpin prints the
	// error in a way that suggests a user problem. For example, an error
	// connecting to Elasticsearch might look like:
	//
	//   cloudwatcher: error: health check timeout: no Elasticsearch node available, try --help

	// Instrument a HTTP client that will be used to connect
	// to Elasticsearch. Don't create the Elasticsearch client
	// itself since the client makes an immeditate call to
	// Elasticsearch to check the connection.
	app.Action(func(*kingpin.ParseContext) error {
		constLabels := map[string]string{"recipient": "elasticsearch"}
		c, err := metrics.InstrumentHTTP(nil, r, namespace, constLabels)
		if err != nil {
			panic("error instrumenting HTTP client: " + err.Error())
		}
		app.clients.ElasticsearchHTTP = c
		return nil
	})

	return app, nil
}

// Main is the main method of App and should be called
// in main.main() after flag parsing.
func (app *App) Main(g prometheus.Gatherer) {
	logger := app.flags.NewLogger()
	defer func() { _ = logger.Sync() }()
	defer cmd.SetGlobalLogger(logger)()

	// Serve the healthchecks, Prometheus metrics, and pprof traces.
	go func() {
		mux := app.flags.ConfigureMux(http.DefaultServeMux, app.health.Handler, g)
		srv := app.flags.NewServer(mux)
		if err := srv.ListenAndServe(); err != nil {
			logger.Fatal("error serving healthchecks/metrics", zap.Error(err))
		}
	}()

	// Set up Elasticsearch client.
	c, err := app.flags.NewElasticsearchClient(
		elastic.SetHttpClient(app.clients.ElasticsearchHTTP),
	)
	if err != nil {
		logger.Fatal("error connecting to Elasticsearch", zap.Error(err))
	}
	defer c.Stop()
	app.clients.Elasticsearch = c
	app.health.ElasticSessionCreated = true

	repo := NewRepositoryService(
		app.clients.Elasticsearch,
		&app.flags.Repository,
		app.flags.DryRun,
	)

	if app.flags.Repository.Type != "" {
		if err := repo.Ensure(context.Background()); err != nil {
			logger.Fatal("error while ensuring snapshot repository", zap.Error(err))
		}
	}

	snapshots, err := repo.ListSnapshots(context.Background())
	if err != nil {
		logger.Fatal("error while listing snapshots", zap.Error(err))
	}
	app.inst.Snapshots.Set(float64(len(snapshots)))

	doBackup := make(chan time.Time)
	go func() {
		doBackup <- time.Now()
		every := app.flags.Config.MinInterval()
		for t := range time.Tick(every) {
			select {
			case doBackup <- t:
			default:
				logger.Warn("skipped backup because previous operation is still running")
				app.inst.SnapshotsSkipped.Inc()
			}
		}
	}()

	timer := prometheus.NewTimer(app.inst.SleepSeconds)
	for range doBackup {
		timer.ObserveDuration() // Observe sleep duration.

		logger.Info("creating snapshot")
		timer = prometheus.NewTimer(app.inst.SnapshotCreationSeconds)
		t, err := repo.CreateSnapshot(context.Background())
		if err != nil {
			logger.Fatal("error while creating snapshot", zap.Error(err))
		}
		timer.ObserveDuration()
		app.inst.SnapshotsCreated.Inc()
		logger.Debug("finished creating snapshot",
			zap.String("snapshot", t.Format(SnapshotFormat)))

		snapshots, err := repo.ListSnapshots(context.Background())
		if err != nil {
			logger.Fatal("error while listing snapshots", zap.Error(err))
		}
		app.inst.Snapshots.Set(float64(len(snapshots)))

		if app.flags.Delete {
			toDelete := retention.Delete(
				app.flags.Config,
				snapshots,
			)
			for _, s := range toDelete {
				n := s.Format(SnapshotFormat)
				logger.Info("deleting snapshot",
					zap.String("snapshot", n))
				timer = prometheus.NewTimer(app.inst.SnapshotDeletionSeconds)
				err := repo.DeleteSnapshot(context.Background(), s)
				if err != nil {
					logger.Fatal("error while deleting snapshot",
						zap.String("snapshot", n),
						zap.Error(err))
				}
				timer.ObserveDuration()
				app.inst.SnapshotsDeleted.Inc()
				logger.Debug("finshed deleting snapshot",
					zap.String("snapshot", n))
			}
		}

		app.inst.Loops.Inc()
		logger.Debug("sleeping")
		timer = prometheus.NewTimer(app.inst.SleepSeconds)
	}
}
