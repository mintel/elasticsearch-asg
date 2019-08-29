package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/heptiolabs/healthcheck"
	elastic "github.com/olivere/elastic/v7"  // Elasticsearch client
	"go.uber.org/zap"                        // Logging
	kingpin "gopkg.in/alecthomas/kingpin.v2" // Command line arg parser.

	// AWS clients and stuff.
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/sqs"

	esasg "github.com/mintel/elasticsearch-asg"         // Complex Elasticsearch services
	"github.com/mintel/elasticsearch-asg/cmd"           // Common logging setup func
	"github.com/mintel/elasticsearch-asg/metrics"       // Prometheus metrics
	"github.com/mintel/elasticsearch-asg/pkg/es"        // Elasticsearch client extensions
	"github.com/mintel/elasticsearch-asg/pkg/lifecycle" // Handle AWS Autoscaling Group lifecycle hook event messages.
	"github.com/mintel/elasticsearch-asg/pkg/squeues"   // SQS message dispatcher
)

// Request retry count/timeouts.
const (
	awsMaxRetries = 3
	esRetryInit   = 150 * time.Millisecond
	esRetryMax    = 1200 * time.Millisecond
)

const subsystem = "lifecycler"

// defaultURL is the default Elasticsearch URL.
const defaultURL = "http://localhost:9200"

// Command line opts
var (
	queueURL      = kingpin.Arg("queue", "URL of SQS queue receiving lifecycle hook events.").Required().String()
	esURL         = kingpin.Arg("url", "Elasticsearch URL. Default: "+defaultURL).Default(defaultURL).URL()
	metricsListen = kingpin.Flag("metrics.listen", "Address on which to expose Prometheus metrics.").Default(":9701").String()
	metricsPath   = kingpin.Flag("metrics.path", "Path under which to expose Prometheus metrics.").Default("/metrics").String()
)

// The Elasticsearch API only allows the clearing of all master voting exclusions at once,
// so only one goroutine should clean up the exclusions once all pending lifecycle events have finished.
// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/voting-config-exclusions.html
var (
	// votingLock allows only one call to the Elasticsearch /_cluster/voting_config_exclusions API at once.
	votingLock sync.Mutex

	// votingCount keeps tracking of how many outstanding lifecycle termination
	// events are being handled for Elasticsearch master-eligible nodes.
	// Each goroutine handling the termination of a master-eligible node
	// increments this this by 1 at the start using the `sync/atomic` package,
	// and decrements by one after the lifecycle event completes.
	// If the goroutine is the one that decrements down to 0, it will clear the master
	// voting exclusions list.
	votingCount int32
)

func main() {
	kingpin.CommandLine.Help = "Handle AWS Autoscaling Group Lifecycle hook events for Elasticsearch from an SQS queue."
	kingpin.Parse()

	logger := cmd.SetupLogging()
	defer func() {
		// Make sure any buffered logs get flushed before exiting successfully.
		// This should never happen because lifecycler should never exit successfully,
		// but just in case...
		// Subsequent calls to loger.Fatal() perform their own Sync().
		// See: https://github.com/uber-go/zap/blob/master/FAQ.md#why-include-dedicated-panic-and-fatal-log-levels
		// Do this inside a closure func so that the linter will stop complaining
		// about not checking the error output of Sync().
		_ = logger.Sync()
	}()

	// Make AWS clients.
	region, err := squeues.Region(*queueURL)
	if err != nil {
		logger.Fatal("error parsing SQS URL", zap.Error(err))
	}
	conf := aws.NewConfig().WithRegion(region).WithMaxRetries(awsMaxRetries)
	sess := session.Must(session.NewSession(conf))
	if *cmd.VerboseFlag {
		// If verbose mode is on, add Prometheus metrics for AWS API call duration and errors.
		prometheus.WrapRegistererWithPrefix(
			prometheus.BuildFQName(metrics.Namespace, "", subsystem),
			prometheus.DefaultRegisterer,
		).MustRegister(metrics.InstrumentAWS(&sess.Handlers, nil)...) // TODO: Define better buckets.
	}
	asClient := autoscaling.New(sess)
	sqsClient := sqs.New(sess)

	// Make Elasticsearch client.
	esClient, err := elastic.DialContext(
		context.Background(),
		elastic.SetURL((*esURL).String()),
		elastic.SetRetrier(elastic.NewBackoffRetrier(elastic.NewExponentialBackoff(esRetryInit, esRetryMax))),
	)
	if err != nil {
		logger.Fatal("error creating Elasticsearch client", zap.Error(err))
	}

	// Setup healthchecks
	health := healthcheck.NewMetricsHandler(prometheus.DefaultRegisterer, prometheus.BuildFQName(metrics.Namespace, "", subsystem))
	health.AddLivenessCheck("up", func() error {
		return nil
	})
	var lastErr error
	health.AddReadinessCheck("noerror", func() error {
		return lastErr
	})

	// Serve health checks and Prometheus metrics.
	go func() {
		http.Handle(*metricsPath, promhttp.Handler())
		http.HandleFunc("/live", health.LiveEndpoint)
		http.HandleFunc("/ready", health.ReadyEndpoint)
		if err := http.ListenAndServe(*metricsListen, nil); err != nil {
			logger.Fatal("error serving metrics", zap.Error(err))
		}
	}()

	// queue will consume messages from an SQS to which Autoscaling Group lifecycle hook messages are published.
	// It will call handlerFn (below) for each message, updating the message's visibility timeout
	// for as long as handleFn takes to run.
	// See: https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-visibility-timeout.html
	queue := squeues.New(sqsClient, *queueURL)

	// handlerFn will be called for each Autoscaling Group lifecycle hook event message received.
	handlerFn := func(ctx context.Context, m *sqs.Message) error {

		// Decode JSON message to Event object.
		event, err := lifecycle.NewEventFromMsg(ctx, asClient, []byte(*m.Body))
		if err == lifecycle.ErrTestEvent {
			// AWS sends an initial test event when the lifecycle hook is first set up.
			// Ignore these.
			return nil
		} else if err != nil {
			// Got an error.
			// Log the raw message at debug level.
			logger.Debug("got sqs message", zap.String("body", *m.Body))
			return err
		}

		// Add event information to the local logger.
		logger := logger.With(
			zap.String("autoscaling_group_name", event.AutoScalingGroupName),
			zap.String("lifecycle_hook_name", event.LifecycleHookName),
			zap.String("instance_id", event.InstanceID),
		)
		logger.Info(
			"got lifecycle event message",
			zap.String("lifecycle_transition", event.LifecycleTransition.String()),
			zap.Time("start_time", event.Start),
		)

		// Keep track of how many times we've delayed the lifecycle event from timing out.
		// Used for error handling later.
		startHeartbeatCount := event.HeartbeatCount

		// cond is the func that determins if the lifecycle event is allowed to proceed or not.
		// It's different depending on if the instance is launching or terminating.
		var cond func(ctx context.Context, e *lifecycle.Event) (bool, error)

		// cleanupFns is a set of functions to be called at the end of this handler.
		// We're using this instead of `defer` because we don't want to run these if there was an error.
		cleanupFns := make([]func(), 0)

		if event.LifecycleTransition == lifecycle.TransitionLaunching {
			cond = launchCondition(logger, esClient)

		} else if event.LifecycleTransition == lifecycle.TransitionTerminating {
			cond = terminateCondition(logger, esClient)

			// Do some initial work if the instance is terminating.
			// First, fetch info about this Elasticsearch node.
			n, err := esasg.NewElasticsearchQueryService(esClient).Node(ctx, event.InstanceID)
			if err != nil {
				return err
			} else if n == nil {
				logger.Info("node has already left Elasticsearch cluster")
				return nil
			}

			esCommand := esasg.NewElasticsearchCommandService(esClient)

			// Drain any index shards from node by settings a shard allocation exclusion.
			if err := esCommand.Drain(ctx, n.Name); err != nil {
				return err
			}
			cleanupFns = append(cleanupFns, func() { // Clean up the exclusion once the node is dead.
				logger.Debug("cleaning up shard allocation exclusions settings")
				if err := esCommand.Undrain(ctx, n.Name); err != nil {
					logger.Fatal("error undraining node", zap.Error(err))
				}
			})

			// If node is a master-eligible node, exclude it from master voting.
			if n.IsMaster() { // Has "master" role.
				logger.Debug("setting master voting exclusion")
				votingLock.Lock()
				if _, err := es.NewClusterPostVotingConfigExclusion(esClient).Node(n.Name).Do(ctx); err != nil {
					votingLock.Unlock()
					return err
				}
				atomic.AddInt32(&votingCount, 1)
				votingLock.Unlock()
				cleanupFns = append(cleanupFns, func() {
					votingLock.Lock()
					defer votingLock.Unlock()
					// Clean up master voting exclusions, but only if this is the only
					// lifecycle event currently being handled.
					if atomic.AddInt32(&votingCount, -1) == 0 {
						logger.Debug("clearing voting exclusion configuration")
						if _, err := es.NewClusterDeleteVotingConfigExclusion(esClient).Do(ctx); err != nil {
							logger.Fatal("error clearing voting exclusion configuration", zap.Error(err))
						}
					}
				})
			}
		} else {
			panic("unknown lifecycle transition: " + event.LifecycleTransition.String())
		}

		// Call lifecycle.KeepAlive(), which will prevent the lifecycle event
		// from timing out if `cond` returns false.
		logger.Debug("waiting for condition to pass")
		err = lifecycle.KeepAlive(ctx, asClient, event, cond)

		if err == lifecycle.ErrExpired {
			// Lifecycle event has expired.
			// This might be an old event that somehow never got deleted from the SQS queue.
			// Just delete the SQS message and move on.
			logger.Info("lifecycle event already expired")
			err = nil

		} else if aerr, ok := err.(awserr.Error); ok && strings.Contains(aerr.Message(), "No active Lifecycle Action found with token") {
			// lifecycler should be able to tell when an event has timed out.
			// This error is thrown if lifecycler called RecordLifecycleActionHeartbeat(), but the
			// event already expired; it should never happen.
			logger.Panic("lifecycle event expired, but lifecycler somehow didn't know")

		} else if err != nil && event.HeartbeatCount != startHeartbeatCount {
			// There was some other kind of error.
			// We want lifecycle to be able to pick up where it left off if restarted.
			// Lifecycle event messages only contain the event start time; we calculate the timeout of the event
			// by calling the `DescribeLifecycleHooks` AWS API, and adding the hook's timeout to the event start time.
			// AFAIK AWS doesn't provide a way to tell how many times the lifecycle event heartbeat has been recorded,
			// which means the next generation of lifecycler won't know when the actual timeout is.
			// We get around that by deleting the original message and sending a new one to the
			// SQS queue that contains the original JSON message + a count of how many times the heartbeat
			// has been recorded.
			originalErr := err
			logger.Info("error during KeepAlive; sending message back to queue", zap.Error(originalErr))
			body, err := json.Marshal(event)
			if err != nil {
				logger.Panic("error serializing event to send back to SQS", zap.Error(err))
			}
			_, err = sqsClient.SendMessage(&sqs.SendMessageInput{ // Send new message.
				QueueUrl:     queueURL,
				MessageBody:  aws.String(string(body)),
				DelaySeconds: aws.Int64(5), // Delay the message a little so this lifecycler doesn't pick it up before exiting.
			})
			if err != nil {
				logger.Panic("error sending message back to SQS", zap.Error(err))
			}
			_, err = sqsClient.DeleteMessage(&sqs.DeleteMessageInput{ // Delete old message.
				QueueUrl:      queueURL,
				ReceiptHandle: m.ReceiptHandle,
			})
			if err != nil {
				logger.Panic("error deleting message from SQS", zap.Error(err))
			}
			return originalErr

		} else if err == nil { // No error. Yay!
			logger.Info("completed lifecycle event successfully")
			// Run cleanup functions (now we can use defer).
			for _, f := range cleanupFns {
				defer f()
			}
		}

		return err
	}

	// Dispatch SQS messages to handlerFn.
	if err = queue.Run(squeues.FuncHandler(handlerFn)); err != nil {
		logger.Fatal("error handling SQS lifecycle messages", zap.Error(err))
	}
}

// terminateCondition returns function that checks if a AWS Autoscaling Group scale down event
// should be allowed to proceed.
func terminateCondition(logger *zap.Logger, client *elastic.Client) func(context.Context, *lifecycle.Event) (bool, error) {
	return func(ctx context.Context, event *lifecycle.Event) (bool, error) {
		// Check cluster health
		resp, err := client.ClusterHealth().Do(ctx)
		switch {
		case err != nil:
			return false, err
		case resp.TimedOut:
			return false, errors.New("request to cluster master node timed out")
		case resp.Status == "red":
			// Terminating an node while the cluster state is red is almost certainly a bad idea.
			logger.Info("condition failed: Cluster status is red.")
			return false, nil
		}

		// Wait for all index shards to drain off this node.
		if n, err := esasg.NewElasticsearchQueryService(client).Node(ctx, event.InstanceID); err != nil {
			return false, err
		} else if len(n.Indices()) > 0 {
			logger.Info("condition failed: node still has shards")
			return false, nil
		}

		logger.Info("condition passed")
		return true, nil
	}
}

// launchCondition returns function that checks if a AWS Autoscaling Group scale up event
// should be allowed to proceed.
func launchCondition(logger *zap.Logger, client *elastic.Client) func(context.Context, *lifecycle.Event) (bool, error) {
	return func(ctx context.Context, event *lifecycle.Event) (bool, error) {
		// Check cluster health
		resp, err := client.ClusterHealth().Do(ctx)
		switch {
		case err != nil:
			return false, err
		case resp.TimedOut:
			return false, errors.New("request to cluster master node timed out")
		case resp.RelocatingShards > 0:
			// Shards are being relocated. This is the activity lifecycler was designed to protect.
			logger.Info("condition failed: RelocatingShards > 0")
			return false, nil
		}

		logger.Info("condition passed")
		return true, nil
	}
}
