// Package squeues implements a concurrent dispatcher for AWS SQS queue messages.
package squeues

import (
	"context"
	"sync"
	"time"

	"github.com/cenkalti/backoff" // Backoff/retry algorithms

	// AWS clients and stuff
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
)

// maxMessages is the max number of message that can be received from SQS at once.
// This is a limit of the AWS SQS API.
const maxMessages = 10

var ( // Make these variables, not constants, so they can be changed during tests.

	// DefaultPollTime is default the max duration to wait for messages.
	// The max allow by the AWS SQS API is 20 seconds.
	DefaultPollTime = 20 * time.Second

	// DefaultInitialVisibilityTimeout is the default initial message visibility timeout.
	DefaultInitialVisibilityTimeout = 5 * time.Second

	// DefaultMaxVisibilityTimeout is the default max message visibility timeout.
	DefaultMaxVisibilityTimeout = 60 * time.Second

	// SQS message visibility timeout must be a multiple of this.
	// This is a limit of the AWS SQS API.
	second = time.Second

	// awsCommBuffer is how long before an SQS message visibiilty times out
	// that a request should be sent to increase the timeout i.e. if message's visibility
	// is going to timeout at time t, send a request to update the timeout at t-awsCommBuffer.
	awsCommBuffer = 2 * time.Second

	// sendVisRandomizationFactor is a percent by which message visibility timeouts
	// will be randomly varied to prevent thundering herds.
	sendVisRandomizationFactor = 0.05
)

// ceilSeconds rounds a duration up to the nearest second.
func ceilSeconds(d time.Duration) time.Duration {
	secs := d / second
	if remainder := d % second; remainder > 0 {
		secs++
	}
	return secs * second
}

// numSeconds returns the number of seconds in a duration, rounded down.
func numSeconds(d time.Duration) int64 {
	return int64(d / second)
}

// Dispatcher is a dispatcher for AWS SQS messages, executing a Handler for each message received.
type Dispatcher struct {
	client   sqsiface.SQSAPI
	queueURL string

	// Maximum number of concurrent messages to handle.
	// Zero (the default) is no limit.
	MaxConcurrent int

	// Max seconds to wait for messages.
	// Defaults to DefaultPollTime.
	PollTime time.Duration

	// Initial message visibility timeout.
	// Defaults to DefaultInitialVisibilityTimeout.
	InitialVisibilityTimeout time.Duration

	// Max message visibility timeout.
	// Defaults to DefaultMaxVisibilityTimeout.
	MaxVisibilityTimeout time.Duration
}

// New returns a new Dispatcher.
func New(client sqsiface.SQSAPI, queueURL string) *Dispatcher {
	return &Dispatcher{
		client:                   client,
		queueURL:                 queueURL,
		PollTime:                 DefaultPollTime,
		InitialVisibilityTimeout: DefaultInitialVisibilityTimeout,
		MaxVisibilityTimeout:     DefaultMaxVisibilityTimeout,
	}
}

// Run consumes messages from the SQS queue and calls h.Handle() as a goroutine for each.
//
// While the Handler is running, the message will periodically have its visibility timeout updated
// to keep the message reserved. If the Handler or any communication with AWS returns a error,
// the context passed to the Handler will be canceled and the error returned.
// If the Handler returns without error, the message will be deleted from SQS.
//
// Unless there is an error, Run() will block forever. If you need the ability to cancel,
// use RunWithContext().
func (q *Dispatcher) Run(handler Handler) error {
	return q.RunWithContext(context.Background(), handler)
}

// RunWithContext is the same as Run() but passing a context.
func (q *Dispatcher) RunWithContext(ctx context.Context, handler Handler) error {
	// Create a ticker that will trigger fetching messages from the SQS queue.
	// Use backoff.Ticker here instead of time.Ticker because time.Ticker drops
	// ticks if nothing is receving on its channel, while backoff.Ticker blocks
	// until the tick is received. Also backoff.Ticker ticks once immediately on instantiation.
	doReceiveMessagesTicker := backoff.NewTicker(backoff.NewConstantBackOff(q.PollTime))
	defer doReceiveMessagesTicker.Stop()
	doReceiveMessages := doReceiveMessagesTicker.C // Refer to the ticker channel in a separate var so we can enable/disable it.
	var receiveMessagesResults chan []*sqs.Message // Channel for the results of receiving SQS messages.

	// Map to keep track of messages pending Handler completion.
	// The map keys are set from sqs.Message.ReceiptHandle.
	// The maps values are a func that will stop the periodic updates to the SQS message visibility.
	pending := make(map[string]func())

	doUpdateMessageVisibility := make(chan changeMessageVisibility)

	// Channel for messages that were handled successfully and need to be deleted.
	// The string values are from sqs.Message.ReceiptHandle.
	doDeleteMessage := make(chan string)

	wg := sync.WaitGroup{} // Keeps track of goroutines to make sure they exit before returning.
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()  // Cancel when RunWithContext returns (i.e. in case of error), so all running Handlers abort.
		wg.Wait() // Wait for goroutines to stop.
	}()

	errc := make(chan error) // Channel for goroutines to report errors to RunWithContext.

	// abortIfErr ends RunWithContext if it receives a non-nil error.
	// It returns true if err was nil, else false.
	// When RunWithContext returns ctx will be cancel, causing all
	// other calls to abortIfErr to noop.
	abortIfErr := func(err error) bool {
		if err == nil {
			return true
		}
		select {
		case errc <- err:
		case <-ctx.Done():
		}
		return false
	}

	for { // Event loop. Everything within the select cases MUST NOT block.
		select {
		case <-ctx.Done(): // Return on context cancel.
			return ctx.Err()

		case err := <-errc: // On error (sent by abortIfErr), return the error.
			return err

		case <-doReceiveMessages: // Start SQS message receive operation.

			// Disable the doReceiveMessages select case until results return.
			doReceiveMessages = nil

			// Figure out how many SQS messages to request.
			var n int
			if q.MaxConcurrent == 0 { // Unlimited up to the AWS API limit.
				n = maxMessages
			} else {
				n = q.MaxConcurrent - len(pending)
			}
			if n > maxMessages {
				n = maxMessages
			}
			if n <= 0 {
				panic("tried to receive < 1 messages")
			}

			// Start a goroutine to call the AWS receive messages API.
			receiveMessagesResults = make(chan []*sqs.Message, 1)
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				if messages, err := q.receiveMessages(ctx, n); abortIfErr(err) {
					select {
					case receiveMessagesResults <- messages:
					case <-ctx.Done(): // Don't block if ctx is canceled.
					}
				}
			}(n)

		case messages := <-receiveMessagesResults: // End SQS message receive operation.
			receiveMessagesResults = nil
			for _, m := range messages {
				// Start handler goroutines for each message.
				wg.Add(1)
				go func(m *sqs.Message) {
					defer wg.Done()
					if err := handler.Handle(ctx, m); abortIfErr(err) {
						select {
						case doDeleteMessage <- *m.ReceiptHandle:
						case <-ctx.Done(): // Don't block if ctx is canceled.
						}
					}
				}(m)

				// Start message visibility updating goroutines.
				rh := *m.ReceiptHandle
				notifierCtx, stopNotifier := context.WithCancel(ctx)
				pending[rh] = stopNotifier
				wg.Add(1)
				go func() {
					defer wg.Done()
					q.notifyChangeMessageVisibility(notifierCtx, rh, doUpdateMessageVisibility)
				}()
			}

			// If there aren't too many messages pending handle completion,
			// enable another message receive operation.
			if q.MaxConcurrent == 0 || len(pending) < q.MaxConcurrent {
				doReceiveMessages = doReceiveMessagesTicker.C // Enable start receive case
			}

		case notice := <-doUpdateMessageVisibility:
			wg.Add(1)
			go func(notice changeMessageVisibility) {
				defer wg.Done()
				abortIfErr(q.updateMessageVisibility(ctx, notice.ReceiptHandle, notice.NewVisibilityTimeout))
			}(notice)

		case receiptHandle := <-doDeleteMessage: // Delete message when handler completes successfully.
			// Stop the periodic updates to message visibility.
			stopMsgVisibilityUpdates, ok := pending[receiptHandle]
			if !ok {
				panic("trying to delete SQS message that isn't pending")
			}
			stopMsgVisibilityUpdates()
			delete(pending, receiptHandle)

			// If there isn't already a pending message receive operation
			// and there aren't too many messages pending handler completion,
			// enable another message receive operation.
			if receiveMessagesResults == nil && (q.MaxConcurrent == 0 || len(pending) < q.MaxConcurrent) {
				doReceiveMessages = doReceiveMessagesTicker.C // Enable start receive case
			}

			// Start a goroutine to delete the message from SQS.
			wg.Add(1)
			go func(rh string) {
				defer wg.Done()
				abortIfErr(q.deleteMessage(ctx, rh))
			}(receiptHandle)
		}
	}
}

// receiveMessages receives up to numMessages from the AWS SQS queue.
func (q *Dispatcher) receiveMessages(ctx context.Context, numMessages int) ([]*sqs.Message, error) {
	resp, err := q.client.ReceiveMessageWithContext(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(q.queueURL),
		MaxNumberOfMessages: aws.Int64(int64(numMessages)),
		VisibilityTimeout:   aws.Int64(numSeconds(ceilSeconds(q.InitialVisibilityTimeout))),
		WaitTimeSeconds:     aws.Int64(numSeconds(q.PollTime)),
	})
	if err != nil {
		return nil, err
	}
	return resp.Messages, nil
}

func (q *Dispatcher) deleteMessage(ctx context.Context, receiptHandle string) error {
	_, err := q.client.DeleteMessageWithContext(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(q.queueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	return err
}

// updateMessageVisibility updates the message visibility timeout for
// the AWS SQS message specified by receiptHandle to d (rounded up to the nearest second).
func (q *Dispatcher) updateMessageVisibility(ctx context.Context, receiptHandle string, d time.Duration) error {
	visibilityTimeout := numSeconds(ceilSeconds(d))
	_, err := q.client.ChangeMessageVisibilityWithContext(ctx, &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          aws.String(q.queueURL),
		ReceiptHandle:     aws.String(receiptHandle),
		VisibilityTimeout: aws.Int64(visibilityTimeout),
	})
	return err
}

// notifyChangeMessageVisibility periodically sends notices over channel c that the visibility timeout
// of the SQS message identified by receiptHandle is about to expire and needs to be updated.
// It will keep sending notifications until ctx is canceled.
// If the notice isn't received in a timely fashion, notifyChangeMessageVisibility will panic.
func (q *Dispatcher) notifyChangeMessageVisibility(ctx context.Context, receiptHandle string, c chan<- changeMessageVisibility) {
	boff := q.backoff()
	visibilityTimeout := boff()

	// Send the notice awsCommBuffer before the next timeout to give
	// Dispatcher time to request AWS update the visibility timeout.
	doSendNotice := time.NewTimer(visibilityTimeout - awsCommBuffer)
	defer doSendNotice.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-doSendNotice.C:
			visibilityTimeout = boff()
			notice := changeMessageVisibility{receiptHandle, visibilityTimeout}

			select {
			case c <- notice: // Send the notice.
			case <-ctx.Done(): // Abort if context is canceled.
				return
			case <-time.After(awsCommBuffer):
				// Notice wasn't received promptly.
				// By now the SQS message visibiilty timeout will have expired.
				// If we request that AWS update the message visibility now, it will error.
				// This shouldn't happen.
				panic("change message visibility notification wasn't received in time")
			}

			doSendNotice.Reset(visibilityTimeout - awsCommBuffer)
		}
	}
}

// backoff returns function that generates increasing visibility timeout durations
// for SQS messages. The first duration will == q.InitialVisibilityTimeout and
// increase exponentially from there to q.MaxVisibilityTimeout.
func (q *Dispatcher) backoff() func() time.Duration {
	// Message visibility timeout should be increased exponentially
	// the longer we hold on to it.
	boff := &backoff.ExponentialBackOff{
		InitialInterval:     q.InitialVisibilityTimeout,
		MaxInterval:         q.MaxVisibilityTimeout,
		RandomizationFactor: sendVisRandomizationFactor,
		Multiplier:          backoff.DefaultMultiplier,
		MaxElapsedTime:      0,
		Clock:               backoff.SystemClock,
	}
	boff.Reset()

	// Skip the first backoff interval.
	// This will == q.InitialVisibilityTimeout +/- some random amount.
	// The first message timeout was set to exactly q.InitialVisibilityTimeout when
	// we called the ReceiveMessages API, so we want the timeout _after_ that.
	_ = boff.NextBackOff()

	next := q.InitialVisibilityTimeout
	return func() time.Duration {
		defer func() {
			next = boff.NextBackOff()
		}()
		return next
	}
}

// changeMessageVisibility notifies a running SQS that a message is about to
// reach its visibility timeout and needs to have that timeout updated.
type changeMessageVisibility struct {
	ReceiptHandle        string        // The message identified by this receipt handle...
	NewVisibilityTimeout time.Duration // should have its visibility timeout updated to this.
}
