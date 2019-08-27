package healthchecker

import (
	"os"
	"path/filepath"

	elastic "github.com/olivere/elastic/v7"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/mintel/elasticsearch-asg/internal/pkg/cmd"
	"github.com/mintel/elasticsearch-asg/internal/pkg/metrics"
)

const (
	Name  = "healthchecker"
	Usage = "Serve liveness and readiness checks for Elasticsearch."
)

// App holds application state.
type App struct {
	*kingpin.Application

	flags  *Flags        // Command line flags
	health *Healthchecks // healthchecks HTTP handler

	// API clients.
	clients struct {
		Elasticsearch *elastic.Client
	}
}

// NewApp returns a new App.
func NewApp(r prometheus.Registerer) (*App, error) {
	namespace := cmd.BuildPromFQName("", Name)

	app := &App{
		Application: kingpin.New(filepath.Base(os.Args[0]), Usage),
		health:      NewHealthchecks(r, namespace),
	}
	app.flags = NewFlags(app.Application)

	// Add action to set up Elasticsearch client after
	// flags are parsed.
	app.Action(func(*kingpin.ParseContext) error {
		constLabels := map[string]string{"recipient": "elasticsearch"}
		httpClient, err := metrics.InstrumentHTTP(nil, r, namespace, constLabels)
		if err != nil {
			return err
		}
		opts := app.flags.ElasticsearchConfig(
			elastic.SetHttpClient(httpClient),
		)
		c, err := elastic.NewClient(opts...)
		if err != nil {
			return err
		}
		app.clients.Elasticsearch = c
		app.health.ElasticSessionCreated = true

		if !app.flags.DisableCheckHead {
			app.health.Handler.AddLivenessCheck(
				"elasticsearch-HEAD",
				CheckLiveHEAD(app.clients.Elasticsearch),
			)
		}
		if !app.flags.DisableCheckJoined {
			app.health.Handler.AddReadinessCheck(
				"joined-cluster",
				CheckReadyJoinedCluster(app.clients.Elasticsearch),
			)
		}
		if !app.flags.DisableCheckRollingUpgrade {
			app.health.Handler.AddReadinessCheck(
				"rolling-upgrade",
				CheckReadyRollingUpgrade(app.clients.Elasticsearch),
			)
		}

		return nil
	})

	return app, nil
}

// Main is the main method of App and should be called
// in main.main() after flag parsing.
func (app *App) Main(g prometheus.Gatherer) {
	logger := app.flags.Logger()
	defer func() { _ = logger.Sync() }()
	defer cmd.SetGlobalLogger(logger)()

	// Serve the healthchecks, Prometheus metrics, and pprof traces.
	mux := app.flags.ConfigureMux(nil, app.health.Handler, g)
	srv := app.flags.Server(mux)
	if err := srv.ListenAndServe(); err != nil {
		logger.Fatal("error serving healthchecks/metrics",
			zap.Error(err))
	}
}
