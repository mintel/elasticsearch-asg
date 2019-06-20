package main

import (
	"context"
	"encoding/json"
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
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/sqs"

	esasg "github.com/mintel/elasticsearch-asg"
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
	queueURL = kingpin.Arg("queue", "URL of SQS queue receiving lifecycle hook events.").Required().String()
	esURL    = kingpin.Arg("url", "Elasticsearch URL. Default: "+defaultURL).Default(defaultURL).URL()
)

var (
	// The Elasticsearch API only allows the clearing of all master voting exclusions at once,
	// so only one goroutine should clean up the exclusions once all pending lifecycle events have finished.
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

	region, err := SQSRegion(*queueURL)
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
	esQuery := esasg.NewElasticsearchQueryService(esClient)
	esCommand := esasg.NewElasticsearchCommandService(esClient)

	queue := squeues.New(sqsClient, *queueURL)
	err = queue.Run(ctx, func(ctx context.Context, m *sqs.Message) error {
		event, err := lifecycle.NewEventFromMsg(ctx, asClient, []byte(*m.Body))
		if err == lifecycle.ErrTestEvent {
			return nil
		} else if err != nil {
			logger.Debug("got sqs message", zap.String("body", *m.Body))
			return err
		}
		startHeartbeatCount := event.HeartbeatCount
		ctx = WithEvent(ctx, event)
		logger := Logger(ctx)
		logger.Info(
			"got lifecycle event message",
			zap.String("lifecycle_transition", event.LifecycleTransition.String()),
			zap.Time("start_time", event.Start),
		)

		var cond func(ctx context.Context, e *lifecycle.Event) (bool, error)
		cleanupFns := make([]func(), 0) // Don't use defer because we don't want to run these in case of error.
		if event.LifecycleTransition == lifecycle.TransitionLaunching {
			cond = launchCondition(esClient)
		} else {
			cond = terminateCondition(esClient)

			n, err := esQuery.Node(ctx, event.InstanceID)
			if err != nil {
				return err
			}
			if n == nil {
				logger.Info("node has already left Elasticsearch cluster")
				return nil
			}

			// Drain shards from node
			if err := esCommand.Drain(ctx, n.Name); err != nil {
				return err
			}
			cleanupFns = append(cleanupFns, func() {
				logger.Debug("cleaning up shard allocation exclusions settings")
				if err := esCommand.Undrain(ctx, n.Name); err != nil {
					logger.Fatal("error undraining node", zap.Error(err))
				}
			})

			// Exclude node from master voting
			if n.IsMaster() {
				logger.Debug("setting master voting exclusion")
				votingLock.Lock()
				if err := esCommand.ExcludeMasterVoting(ctx, n.Name); err != nil {
					votingLock.Unlock()
					return err
				}
				atomic.AddInt32(&votingCount, 1)
				votingLock.Unlock()
				cleanupFns = append(cleanupFns, func() {
					votingLock.Lock()
					defer votingLock.Unlock()
					if atomic.AddInt32(&votingCount, -1) == 0 {
						logger.Debug("clearing voting exclusion configuration")
						if err := esCommand.ClearMasterVotingExclusions(ctx); err != nil {
							logger.Fatal("error clearing voting exclusion configuration", zap.Error(err))
						}
					}
				})
			}
		}

		logger.Debug("waiting for condition to pass")
		err = lifecycle.KeepAlive(ctx, asClient, event, cond)
		if err == lifecycle.ErrExpired {
			// Lifecycle event has expired. Just delete the SQS message.
			logger.Info("lifecycle event already expired")
			err = nil

		} else if aerr, ok := err.(awserr.Error); ok && strings.Contains(aerr.Message(), "No active Lifecycle Action found with token") {
			// lifecycler should be able to tell when an event has timed out.
			// This error is thrown if lifecycler called RecordLifecycleActionHeartbeat(), but the
			// event already expired; it should never happen.
			logger.Panic("lifecycle event expired, but lifecycler somehow didn't know")

		} else if err != nil && event.HeartbeatCount != startHeartbeatCount {
			// There's no AWS built-in way AFAIK to tell how many times the heartbeat has been recorded.
			// We'll get around that by deleting the original message and sending a new one to the
			// queue that contains the HeartbeatCount.
			logger.Info("error during KeepAlive; sending message back to queue", zap.Error(err))
			body, err := json.Marshal(event)
			if err != nil {
				logger.Panic("error serializing event to send back to SQS", zap.Error(err))
			}
			_, err = sqsClient.SendMessage(&sqs.SendMessageInput{
				QueueUrl:    queueURL,
				MessageBody: aws.String(string(body)),
			})
			if err != nil {
				logger.Panic("error sending message back to SQS", zap.Error(err))
			}
			_, err = sqsClient.DeleteMessage(&sqs.DeleteMessageInput{
				QueueUrl:      queueURL,
				ReceiptHandle: m.ReceiptHandle,
			})
			if err != nil {
				logger.Panic("error deleting message from SQS", zap.Error(err))
			}

		} else if err == nil {
			logger.Info("completed lifecycle event successfully")
			for i := len(cleanupFns) - 1; i >= 0; i-- {
				cleanupFns[i]()
			}
		}
		return err
	})
	if err != nil {
		logger.Fatal("error handling SQS lifecycle messages", zap.Error(err))
	}
}

// SQSRegion parses an SQS queue URL to return the AWS region its in.
func SQSRegion(URL string) (string, error) {
	u, err := url.Parse(URL)
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
