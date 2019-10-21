package drainer

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/olebedev/emitter"                    // Event bus.
	elastic "github.com/olivere/elastic/v7"          // Elasticsearch client.
	"github.com/pkg/errors"                          // Wrap errors with stacktrace.
	"github.com/prometheus/client_golang/prometheus" // Prometheus metrics.
	"go.uber.org/zap"                                // Logging.
	"golang.org/x/sync/errgroup"                     // Cancel multiple goroutines if one fails.
	kingpin "gopkg.in/alecthomas/kingpin.v2"         // Command line flag parsing.

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/sqsiface"

	"github.com/mintel/elasticsearch-asg/internal/pkg/cmd"     // Common command line app tools.
	"github.com/mintel/elasticsearch-asg/internal/pkg/metrics" // Prometheus metrics tools.
	"github.com/mintel/elasticsearch-asg/pkg/events"           // AWS CloudWatch Events.
)

const (
	Name  = "drainer"
	Usage = "Remove shards from Elasticsearch nodes on EC2 instances that are about to be terminated."

	_nodeAdded   = "node-added"
	_nodeEmpty   = "node-empty"
	_nodeRemoved = "node-removed"
)

// App holds application state.
type App struct {
	*kingpin.Application

	flags  *Flags           // Command line flags
	health *Healthchecks    // healthchecks HTTP handler
	inst   *Instrumentation // App-specific Prometheus metrics

	// API clients.
	clients struct {
		ElasticsearchHTTP   *http.Client
		Elasticsearch       *elastic.Client
		ElasticsearchFacade *ElasticsearchFacade

		SQS         sqsiface.ClientAPI
		AutoScaling autoscalingiface.ClientAPI
	}

	clusterStateMu sync.RWMutex
	clusterState   *ClusterState

	events    *emitter.Emitter
	postponer *LifecycleActionPostponer
}

// NewApp returns a new App.
func NewApp(r prometheus.Registerer) (*App, error) {
	namespace := cmd.BuildPromFQName("", Name)

	app := &App{
		Application: kingpin.New(filepath.Base(os.Args[0]), Usage),
		health:      NewHealthchecks(r, namespace),
		events:      emitter.New(10),
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

	// Add action to set up AWS client(s) after
	// flags are parsed.
	app.Action(func(*kingpin.ParseContext) error {
		cfg := app.flags.AWSConfig()
		err := metrics.InstrumentAWS(&cfg.Handlers, r, namespace, nil)
		if err != nil {
			panic("error instrumenting AWS config: " + err.Error())
		}
		app.clients.SQS = sqs.New(cfg)
		app.clients.AutoScaling = autoscaling.New(cfg)
		app.postponer = NewLifecycleActionPostponer(app.clients.AutoScaling)
		app.health.AWSSessionCreated = true
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
			logger.Fatal("error serving healthchecks/metrics",
				zap.Error(err))
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
	app.clients.ElasticsearchFacade = NewElasticsearchFacade(c)
	app.health.ElasticSessionCreated = true

	eg, ctx := errgroup.WithContext(context.Background())

	// Poll Elasticsearch once at the beginning so we have some
	// idea of the current state.
	if err := app.updateClusterState(ctx); err != nil {
		logger.Fatal("error getting cluster state",
			zap.Error(err))
	}

	// Start polling Elasticsearch for status updates.
	eg.Go(func() error {
		for range time.Tick(app.flags.PollInterval) {
			if err := app.updateClusterState(ctx); err != nil {
				return err
			}
			logger.Debug("updated cluster state")
			app.inst.PollTotal.Inc()
		}
		return nil
	})

	// Start consuming CloudWatch events from SQS.
	eg.Go(func() error {
		e := NewCloudWatchEventEmitter(
			app.clients.SQS,
			app.flags.Queue.String(),
			app.events,
		)
		e.Received = app.inst.MessagesReceived
		return e.Run(ctx)
	})

	// Batch many spot interruptions together.
	// That way we don't hit the Elasticsearch cluster settings API
	// many times if lots of instances get interrupted all at once.
	spotInterruptionEvents := batchEvents(
		app.events.On(
			topicKey("aws.ec2", "EC2 Spot Instance Interruption Warning"),
		),
		make(chan []emitter.Event, 1), // Channel for the batches.
		10*time.Millisecond,           // Wait 10ms for more events to come in before returning batch.
		20,                            // Batch size of at most 20.
	)

	// We shouldn't need to batch autoscaling group termination actions
	// because an AutoScaling group will only ever terminate one instance
	// at a time.
	autoscalingTerminationEvents := app.events.On(
		topicKey("aws.autoscaling", "EC2 Instance-terminate Lifecycle Action"),
	)

loop:
	for {
		select {
		case <-ctx.Done():
			break loop

		case batch, ok := <-spotInterruptionEvents:
			app.inst.MessagesReceived.Add(float64(len(batch)))
			app.inst.SpotInterruptions.Add(float64(len(batch)))
			if !ok {
				logger.Panic("event listener closed")
			}
			cwes := make([]*events.CloudWatchEvent, len(batch))
			for i, e := range batch {
				cwes[i] = e.Args[0].(*events.CloudWatchEvent)
				logger.Info("got spot interruption cloudwatch event",
					zap.String("instance", cwes[i].Detail.(*events.EC2SpotInterruption).InstanceID))
			}
			eg.Go(func() error {
				return app.handleSpotInterruptionEvent(ctx, cwes)
			})

		case e, ok := <-autoscalingTerminationEvents:
			app.inst.MessagesReceived.Inc()
			app.inst.TerminationHookActionsTotal.Inc()
			if !ok {
				logger.Panic("event listener closed")
			}
			cwe := e.Args[0].(*events.CloudWatchEvent)
			logger.Info("got lifecycle termination action cloudwatch event",
				zap.String("instance", cwe.Detail.(*events.AutoScalingLifecycleTerminateAction).EC2InstanceID))
			eg.Go(func() error {
				app.inst.TerminationHookActionsInProgress.Inc()
				defer app.inst.TerminationHookActionsInProgress.Dec()
				return app.handleLifecycleTerminateActionEvent(ctx, cwe)
			})
		}
	}

	if err := eg.Wait(); err != nil {
		logger.Fatal("error in goroutine",
			zap.Error(err))
	}
}

// handleSpotInterruptionEvent handles a spot instance interruption notice from
// CloudWatch events by draining the node. It's highly unlikely that the 2 minutes
// warning we get for spot interruptions is enough to fully drain the node, but it
// is enough time for Elasticsearch to promote other shards to primary.
func (app *App) handleSpotInterruptionEvent(ctx context.Context, batch []*events.CloudWatchEvent) error {
	ids := make([]string, len(batch))
	for i, e := range batch {
		d := e.Detail.(*events.EC2SpotInterruption)
		ids[i] = d.InstanceID
	}
	return app.clients.ElasticsearchFacade.DrainNodes(ctx, ids)
}

// handleLifecycleTerminateActionEvent handles an AutoScaling Group Termination Lifecycle
// Hook event by:
//
// - Draining the node.
// - Waiting for the node to be drained.
func (app *App) handleLifecycleTerminateActionEvent(ctx context.Context, e *events.CloudWatchEvent) error {
	a, err := NewLifecycleAction(e)
	if err != nil {
		return err
	}

	zap.L().Info("draining node", zap.String("node", a.InstanceID))
	err = app.clients.ElasticsearchFacade.DrainNodes(ctx, []string{a.InstanceID})
	if err != nil {
		return err
	}

	// Wait until an event arrives indicating that the node is either
	// empty of all shards or has left the cluster for some reason.
	// While waiting, periodically record a heartbeat for the lifecycle action
	// so it doesn't timeout.

	// Add a global cancel to the context so we can stop everything in
	// case of an error.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Use this context/cancel to stop recording the lifecycle action heartbeat
	// once the drained/removed event comes in.
	postponeCtx, postponeCancel := context.WithCancel(ctx)
	defer postponeCancel()

	// Wait for the event(s) in a goroutine.
	// Cancel the postponeCtx once it arrives.
	go func() {
		k := topicKey(_nodeEmpty, a.InstanceID)
		emptyChan := app.events.Once(k)
		defer app.events.Off(k, emptyChan)

		k = topicKey(_nodeRemoved, a.InstanceID)
		removedChan := app.events.Once(k)
		defer app.events.Off(k, removedChan)

		defer postponeCancel()
		select {
		case <-postponeCtx.Done():
			// This might happend if the lifecycle action global
			// timeout is reached.
			return

		case _, ok := <-emptyChan:
			if ok {
				zap.L().Debug("node empty",
					zap.String("node", a.InstanceID))
			} else {
				zap.L().DPanic("node empty channel was closed",
					zap.String("node", a.InstanceID))
			}

		case _, ok := <-removedChan:
			if ok {
				zap.L().Debug("node removed from cluster",
					zap.String("node", a.InstanceID))
			} else {
				zap.L().DPanic("node removed channel was closed",
					zap.String("node", a.InstanceID))
			}
		}
	}()

	// Record the lifecycle action heartbeat.
	err = app.postponer.Postpone(
		postponeCtx,
		app.clients.AutoScaling,
		a,
	)
	switch err {
	case nil, context.Canceled:
		select {
		case <-ctx.Done():
			// Something external canceled the the context that was
			// originally passed in.
			return ctx.Err()
		case <-postponeCtx.Done():
			// A drained/removed event came in. Continue.
		default:
			zap.L().Panic("postponer return canceled error without context actually being canceled")
		}
	case ErrLifecycleActionTimeout:
		// The lifecycle action reached it's global timeout.
		// This probably shouldn't happen, but it's
		// not a reason to stop the world.
		zap.L().Error("lifecycle action timed out",
			zap.Error(err))
		return nil
	default:
		// Some other error happend while recording the lifecycle
		// action heartbeat.
		return err
	}

	// Complete the autoscaling lifecycle termination action,
	// allowing other autoscaling events to happen.
	zap.L().Debug("completing termination lifecycle action",
		zap.String("instance", a.InstanceID))
	req := app.clients.AutoScaling.CompleteLifecycleActionRequest(&autoscaling.CompleteLifecycleActionInput{
		AutoScalingGroupName:  aws.String(a.AutoScalingGroupName),
		LifecycleHookName:     aws.String(a.LifecycleHookName),
		InstanceId:            aws.String(a.InstanceID),
		LifecycleActionToken:  aws.String(a.Token),
		LifecycleActionResult: aws.String("CONTINUE"),
	})
	_, err = req.Send(context.Background())
	if err != nil {
		// It's not really a problem if we can't complete the lifecycle event
		// because it will timeout on its own eventually.
		zap.L().Warn("failed to complete complete termination lifecycle action",
			zap.Error(err))
	}

	return nil
}

// updateState polls Elasticsearch for updated information about the cluster's state,
// and also cleans up shard allocation exclusions for nodes that have left the cluster.
func (app *App) updateClusterState(ctx context.Context) error {
	app.clusterStateMu.Lock()
	defer app.clusterStateMu.Unlock()

	newState, err := app.clients.ElasticsearchFacade.GetState(ctx)
	if err != nil {
		return errors.Wrap(err, "error getting cluster state")
	}

	oldState := app.clusterState
	app.clusterState = newState
	added, removed := oldState.DiffNodes(newState)

	// Clean up drained nodes that are no longer in the cluster.
	var toUndrain []string
	for _, n := range newState.Exclusions.Name {
		if !newState.HasNode(n) {
			toUndrain = append(toUndrain, n)
			removed = append(removed, n)
		}
	}
	if len(toUndrain) != 0 {
		zap.L().Debug("undraining nodes",
			zap.Strings("nodes", toUndrain))
		if err := app.clients.ElasticsearchFacade.UndrainNodes(ctx, toUndrain); err != nil {
			return errors.Wrap(err, "error while undraining nodes")
		}
		removed = uniqStrings(removed...)
	}

	toWait := make(emitWaiter, 0, len(added)+len(removed)+len(newState.Nodes))
	// Emit events for nodes added.
	for _, n := range added {
		zap.L().Debug("emit node added", zap.String("node", n))
		toWait = append(toWait, app.events.Emit(topicKey(_nodeAdded, n)))
	}
	// Emit events for nodes removed.
	for _, n := range removed {
		zap.L().Debug("emit node removed", zap.String("node", n))
		toWait = append(toWait, app.events.Emit(topicKey(_nodeRemoved, n)))
	}
	// Emit events for nodes emptied.
	for _, n := range newState.Nodes {
		if c, ok := newState.Shards[n]; !ok || c == 0 {
			zap.L().Debug("emit node empty", zap.String("node", n))
			toWait = append(toWait, app.events.Emit(topicKey(_nodeEmpty, n)))
		}
	}

	// Wait for events to finish emitting.
	toWait.Wait()

	return nil
}

func uniqStrings(strs ...string) []string {
	out := make([]string, 0, len(strs))
	m := make(map[string]struct{}, len(strs))
	for _, s := range strs {
		if _, ok := m[s]; !ok {
			out = append(out, s)
			m[s] = struct{}{}
		}
	}
	return out
}
