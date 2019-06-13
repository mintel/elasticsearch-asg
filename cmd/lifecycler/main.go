package main

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	elastic "github.com/olivere/elastic/v7"
	"go.uber.org/zap"
	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/sqs"

	esasg "github.com/mintel/elasticsearch-asg"
	"github.com/mintel/elasticsearch-asg/pkg/es"
	"github.com/mintel/elasticsearch-asg/pkg/lifecycle"
	"github.com/mintel/elasticsearch-asg/pkg/squeues"
)

const (
	awsMaxRetries = 3
	esRetryInit   = 150 * time.Millisecond
	esRetryMax    = 1200 * time.Millisecond
)

const defaultURL = "http://localhost:9200"

// Command line opts
var (
	queueURL = kingpin.Arg("queue", "URL of SQS queue.").Required().String()
	esURL    = kingpin.Arg("url", "Elasticsearch URL. Default: "+defaultURL).Default(defaultURL).URL()
)

var (
	// Only one goroutine should modify shards allocation exclusion settings at once
	// because Elasticsearch doesn't provide an atomic way to modify settings.
	drainLock sync.Mutex

	// The Elasticsearch API only allows the clearing of all master voting exclusions at once,
	// so only one goroutine should clean up the exclusions once all pending checks have finished.
	votingLock  sync.Mutex
	votingCount int32
)

func main() {
	kingpin.CommandLine.Help = "Handle AWS Autoscaling Group Lifecycle hook events for Elasticsearch from an SQS queue."
	kingpin.Parse()

	logger := esasg.SetupLogging()
	defer func() {
		err := logger.Sync()
		if err != nil {
			panic(err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	region, err := RegionFromSQSURL(*queueURL)
	if err != nil {
		logger.Fatal("error parsing SQS URL", zap.Error(err))
	}
	conf := aws.NewConfig().WithRegion(region).WithMaxRetries(awsMaxRetries)
	sess := session.Must(session.NewSession(conf))
	asClient := autoscaling.New(sess)
	sqsClient := sqs.New(sess)

	esClient, err := elastic.DialContext(
		ctx,
		elastic.SetURL((*esURL).String()),
		elastic.SetRetrier(elastic.NewBackoffRetrier(elastic.NewExponentialBackoff(esRetryInit, esRetryMax))),
	)
	if err != nil {
		logger.Fatal("error creating Elasticsearch client", zap.Error(err))
	}

	queue := squeues.New(sqsClient, *queueURL)

	err = queue.Run(ctx, func(ctx context.Context, m *sqs.Message) error {
		event, err := lifecycle.NewEventFromMsg(ctx, asClient, []byte(*m.Body))
		if err == lifecycle.ErrTestEvent {
			return nil
		} else if err != nil {
			logger.Debug("got sqs message", zap.String("body", *m.Body))
			return err
		}
		ctx = WithEvent(ctx, event)
		logger := Logger(ctx)
		logger.Info(
			"got lifecycle event message",
			zap.String("lifecycle_transition", event.LifecycleTransition.String()),
			zap.Time("start_time", event.Start),
		)

		var cond func(ctx context.Context, e *lifecycle.Event) (bool, error)
		if event.LifecycleTransition == lifecycle.TransitionLaunching {
			cond = launchCondition(esClient)
		} else {
			cond = terminateCondition(esClient)

			s := esasg.NewElasticsearchService(esClient)
			n, err := s.Node(ctx, event.InstanceID)
			if err != nil {
				return err
			}

			// Drain shards from node
			drainLock.Lock()
			err = s.Drain(ctx, n)
			drainLock.Unlock()
			if err != nil {
				return err
			}
			defer func() {
				drainLock.Lock()
				defer drainLock.Unlock()
				err := s.Undrain(ctx, n)
				if err != nil {
					logger.Fatal("error undraining node", zap.Error(err))
				}
			}()

			// Exclude node from master voting
			if n.IsMaster() {
				votingLock.Lock()
				logger.Debug("settings master voting exclusion")
				if _, err = es.NewClusterPostVotingConfigExclusion(esClient).Node(event.InstanceID).Do(ctx); err != nil {
					votingLock.Unlock()
					return err
				}
				atomic.AddInt32(&votingCount, 1)
				votingLock.Unlock()
				defer func() {
					votingLock.Lock()
					defer votingLock.Unlock()
					if atomic.AddInt32(&votingCount, -1) == 0 {
						_, err := es.NewClusterDeleteVotingConfigExclusion(esClient).Wait(true).Do(ctx)
						if err != nil {
							logger.Fatal("error clearing voting exclusion configuration", zap.Error(err))
						}
					}
				}()
			}
		}

		logger.Debug("waiting for condition to pass")
		return lifecycle.KeepAlive(ctx, asClient, event, cond)
	})
	if err != nil {
		logger.Fatal("error handling SQS lifecycle messages", zap.Error(err))
	}
}

// RegionFromSQSURL parses an SQS queue URL to return the AWS region its in.
func RegionFromSQSURL(qURL string) (string, error) {
	u, err := url.Parse(qURL)
	if err != nil {
		return "", err
	}
	region := strings.Split(u.Host, ".")[1]
	return region, nil
}

func terminateCondition(client *elastic.Client) func(context.Context, *lifecycle.Event) (bool, error) {
	return func(ctx context.Context, event *lifecycle.Event) (bool, error) {
		logger := Logger(ctx)

		// Check cluster health
		resp, err := client.ClusterHealth().Do(ctx)
		switch {
		case err != nil:
			return false, err
		case resp.TimedOut:
			return false, errors.New("request to cluster master node timed out")
		case resp.RelocatingShards > 0:
			logger.Info("condition failed: RelocatingShards > 0")
			return false, nil
		case resp.InitializingShards > 0:
			logger.Info("condition failed: InitializingShards > 0")
			return false, nil
		case resp.DelayedUnassignedShards > 0:
			logger.Info("condition failed: DelayedUnassignedShards > 0")
			return false, nil
		case resp.UnassignedShards > 0:
			logger.Info("condition failed: UnassignedShards > 0")
			return false, nil
		case resp.Status != "green":
			logger.Info("condition failed: Status != 'green'")
			return false, nil
		}

		// Wait for shards to drain
		if n, err := esasg.NewElasticsearchService(client).Node(ctx, event.InstanceID); err != nil {
			return false, err
		} else if len(n.Indices()) > 0 {
			logger.Info("condition failed: node still has shards")
			return false, nil
		}

		logger.Info("condition passed")
		return true, nil
	}
}

func launchCondition(client *elastic.Client) func(context.Context, *lifecycle.Event) (bool, error) {
	return func(ctx context.Context, event *lifecycle.Event) (bool, error) {
		logger := Logger(ctx)

		// Check cluster health
		resp, err := client.ClusterHealth().Do(ctx)
		switch {
		case err != nil:
			return false, err
		case resp.TimedOut:
			return false, errors.New("request to cluster master node timed out")
		case resp.RelocatingShards > 0:
			logger.Info("condition failed: RelocatingShards > 0")
			return false, nil
		case resp.InitializingShards > 0:
			logger.Info("condition failed: InitializingShards > 0")
			return false, nil
		case resp.DelayedUnassignedShards > 0:
			logger.Info("condition failed: DelayedUnassignedShards > 0")
			return false, nil
		case resp.UnassignedShards > 0:
			logger.Info("condition failed: UnassignedShards > 0")
			return false, nil
		case resp.Status != "green":
			logger.Info("condition failed: Status != 'green'")
			return false, nil
		}

		logger.Info("condition passed")
		return true, nil
	}
}

type ctxKey string

const loggerKey ctxKey = "logger"

// WithFields adds a logger to ctx's values with the given fields, or adds
// the fields to an existing logger.
func WithFields(ctx context.Context, fields ...zap.Field) context.Context {
	logger := Logger(ctx)
	return context.WithValue(ctx, loggerKey, logger.With(fields...))
}

// WithEvent adds lifecycle event info to the context.
func WithEvent(ctx context.Context, e *lifecycle.Event) context.Context {
	return WithFields(ctx,
		zap.String("autoscaling_group_name", e.AutoScalingGroupName),
		zap.String("lifecycle_hook_name", e.LifecycleHookName),
		zap.String("instance_id", e.InstanceID),
	)
}

// Logger returns a new logger from the given context.
func Logger(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return zap.L()
	}
	if logger, ok := ctx.Value(loggerKey).(*zap.Logger); ok {
		return logger
	}
	return zap.L()
}
