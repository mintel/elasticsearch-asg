package drainer

import (
	"context"
	"encoding/json"
	"strconv"
	"sync/atomic"

	"github.com/olebedev/emitter"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/sqsiface"

	"github.com/mintel/elasticsearch-asg/pkg/events"
)

// CloudWatchEventEmitter consumes CloudWatch events from an SQS
// queue and emits them as github.com/olebedev/emitter events.
type CloudWatchEventEmitter struct {
	client sqsiface.ClientAPI
	queue  string
	events *emitter.Emitter

	// Counters.
	received int64
	deleted  int64
}

// NewCloudWatchEventEmitter returns a new CloudWatchEventEmitter.
func NewCloudWatchEventEmitter(c sqsiface.ClientAPI, queueURL string, e *emitter.Emitter) *CloudWatchEventEmitter {
	return &CloudWatchEventEmitter{
		client: c,
		queue:  queueURL,
		events: e,
	}
}

// Run receives and emits CloudWatch events until the context is canceled
// or an error occurs.
func (e *CloudWatchEventEmitter) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		default:
			// Receive SQS messages.
			msgs, err := e.receive(ctx)
			if err != nil {
				return err
			}

			// Unmarshal and emit events.
			toWait := make(emitWaiter, 0, len(msgs))
			for _, m := range msgs {
				cwEvent := &events.CloudWatchEvent{}
				if err := json.Unmarshal([]byte(*m.Body), cwEvent); err != nil {
					zap.L().DPanic("error unmarshaling CloudWatch Event",
						zap.Error(err))
					continue
				}
				c := e.events.Emit(topicKey(cwEvent.Source, cwEvent.DetailType), cwEvent)
				toWait = append(toWait, c)
			}

			// Wait for events to be emitted.
			toWait.Wait()

			// Delete SQS messages.
			if err := e.delete(ctx, msgs); err != nil {
				return err
			}
			atomic.AddInt64(&e.deleted, int64(len(msgs)))
		}
	}
}

// receive receives SQS messages.
func (e *CloudWatchEventEmitter) receive(ctx context.Context) ([]sqs.Message, error) {
	req := e.client.ReceiveMessageRequest(&sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(e.queue),
		MaxNumberOfMessages: aws.Int64(10), // Max allowed by the AWS API.
		WaitTimeSeconds:     aws.Int64(20), // Max allowed by the AWS API.
	})
	resp, err := req.Send(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error getting SQS messages")
	}
	atomic.AddInt64(&e.received, int64(len(resp.Messages)))
	return resp.Messages, nil
}

// delete deletes SQS messages.
func (e *CloudWatchEventEmitter) delete(ctx context.Context, msgs []sqs.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	b := make([]sqs.DeleteMessageBatchRequestEntry, len(msgs))
	for i, m := range msgs {
		b[i] = sqs.DeleteMessageBatchRequestEntry{
			Id:            aws.String(strconv.Itoa(i)),
			ReceiptHandle: m.ReceiptHandle,
		}
	}
	req := e.client.DeleteMessageBatchRequest(&sqs.DeleteMessageBatchInput{
		QueueUrl: aws.String(e.queue),
		Entries:  b,
	})
	_, err := req.Send(ctx)
	if err != nil {
		return errors.Wrap(err, "error deleting SQS messages")
	}
	atomic.AddInt64(&e.deleted, int64(len(msgs)))
	return nil
}

func (e *CloudWatchEventEmitter) Received() int64 {
	return atomic.LoadInt64(&e.received)
}

func (e *CloudWatchEventEmitter) Deleted() int64 {
	return atomic.LoadInt64(&e.deleted)
}
