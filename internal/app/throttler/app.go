package throttler

import (
	"os"
	"path/filepath"

	elastic "github.com/olivere/elastic/v7"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/autoscalingiface"

	"github.com/mintel/elasticsearch-asg/internal/pkg/cmd"
	"github.com/mintel/elasticsearch-asg/internal/pkg/metrics"
)

const (
	Name  = "throttler"
	Usage = "Enable or disable AWS AutoScaling Group scaling based on Elasticsearch cluster status."
)

// App holds application state.
type App struct {
	*kingpin.Application

	flags  *Flags           // Command line flags
	health *Healthchecks    // healthchecks HTTP handler
	inst   *Instrumentation // App-specific Prometheus metrics

	// API clients.
	clients struct {
		Elasticsearch *elastic.Client
		AutoScaling   autoscalingiface.ClientAPI
	}
}

// NewApp returns a new App.
func NewApp(r prometheus.Registerer) (*App, error) {
	namespace := cmd.BuildPromFQName("", Name)

	m := NewInstrumentation(namespace)
	if err := r.Register(m); err != nil {
		return nil, err
	}

	app := &App{
		Application: kingpin.New(filepath.Base(os.Args[0]), Usage),
		inst:        m,
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
		svc, err := elastic.NewClient(
			app.flags.ElasticsearchConfig(
				elastic.SetHttpClient(httpClient),
			)...,
		)
		if err != nil {
			return err
		}
		app.clients.Elasticsearch = svc
		app.health.ElasticSessionCreated = true
		return nil
	})

	// Add action to set up AWS client(s) after
	// flags are parsed.
	app.Action(func(*kingpin.ParseContext) error {
		cfg := app.flags.AWSConfig()
		err := metrics.InstrumentAWS(&cfg.Handlers, r, namespace, nil)
		if err != nil {
			return err
		}
		app.clients.AutoScaling = autoscaling.New(cfg)
		app.health.AWSSessionCreated = true
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
	go func() {
		mux := app.flags.ConfigureMux(nil, app.health.Handler, g)
		srv := app.flags.Server(mux)
		if err := srv.ListenAndServe(); err != nil {
			logger.Fatal("error serving healthchecks/metrics", zap.Error(err))
		}
	}()

	clusterState := NewClusterStateGetter(app.clients.Elasticsearch)
	scaling := make(map[string]*AutoScalingGroupEnabler, len(app.flags.AutoScalingGroupNames))
	for _, group := range app.flags.AutoScalingGroupNames {
		enabler, err := NewAutoScalingGroupEnabler(
			app.clients.AutoScaling,
			logger,
			app.flags.DryRun,
			group,
		)
		if err != nil {
			logger.Fatal(
				"error describing AutoScaling Group",
				zap.String("autoscaling_group", group),
				zap.Error(err),
			)
		}
		scaling[group] = enabler
	}

	ticks := app.flags.Tick()
	for range ticks {
		state, err := clusterState.Get()
		if err != nil {
			logger.Fatal("error getting Elasticsearch cluster state", zap.Error(err))
		}

		var good bool
		switch {
		case state.Status == "red":
			// Don't scale when the cluster status is red.
			// To do so risks data loss.
			logger.Debug("cluster status is red")
			good = false
		case state.RelocatingShards:
			// Don't scale when shards are being moved between nodes.
			// If a scaling event just happened, Elasticsearch will be rebalancing
			// shards around, which causes load, which causes another scaling
			// event, etc...
			// Break the cycle by waiting for shards to stop moving before allowing
			// another scaling event to happen.
			logger.Debug("cluster has relocating shards")
			good = false
		case state.RecoveringFromStore:
			// Don't scale when indices are being recovered from data stored
			// on disk. This likely indicates a node has recently been restarted.
			// Let it recover before allowing scaling.
			logger.Debug("cluster has indices recovering from data on-disk")
			good = false
		default:
			logger.Debug("cluster state is good")
			good = true
		}

		if good {
			for group, enabler := range scaling {
				if err := enabler.Enable(); err != nil {
					logger.Fatal(
						"error enabling autoscaling",
						zap.String("autoscaling_group", group),
						zap.Error(err),
					)
				}
				app.inst.ScalingStatus.With(prometheus.Labels{"group": group}).Set(1)
			}
		} else {
			for group, enabler := range scaling {
				if err := enabler.Disable(); err != nil {
					logger.Fatal(
						"error disabling autoscaling",
						zap.String("autoscaling_group", group),
						zap.Error(err),
					)
				}
				app.inst.ScalingStatus.With(prometheus.Labels{"group": group}).Set(0)
			}
		}

		app.inst.Loops.Inc()
	}
}
