package healthchecker

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	elastic "github.com/olivere/elastic/v7"          // Elasticsearch client.
	"github.com/prometheus/client_golang/prometheus" // Prometheus metrics.
	"go.uber.org/zap"                                // Logging.
	kingpin "gopkg.in/alecthomas/kingpin.v2"         // Command line flag parsing.

	"github.com/mintel/elasticsearch-asg/internal/pkg/cmd"     // Common command line app tools.
	"github.com/mintel/elasticsearch-asg/internal/pkg/metrics" // Prometheus metrics tools.
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
		ElasticsearchHTTP *http.Client
		Elasticsearch     *elastic.Client
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
	logger := app.flags.Logger()
	defer func() { _ = logger.Sync() }()
	defer cmd.SetGlobalLogger(logger)()

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

	if app.flags.Once {
		r := httptest.NewRequest("GET", app.flags.ReadyPath, nil)
		w := httptest.NewRecorder()
		app.health.Handler.ReadyEndpoint(w, r)
		if w.Code == 200 {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}

	// Serve the healthchecks, Prometheus metrics, and pprof traces.
	mux := app.flags.ConfigureMux(http.DefaultServeMux, app.health.Handler, g)
	srv := app.flags.Server(mux)
	if err := srv.ListenAndServe(); err != nil {
		logger.Fatal("error serving healthchecks/metrics",
			zap.Error(err))
	}
}
