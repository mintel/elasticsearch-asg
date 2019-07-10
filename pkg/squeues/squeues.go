// Package squeues implements a concurrent dispatcher for AWS SQS queue messages.
package squeues

import (
	"context"
	"time"

	"github.com/cenkalti/backoff"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
)

// Max number of message that can be received from SQS at once.
const maxMessages = 10

var (
	// DefaultPollTime is default the max seconds to wait for messages.
	DefaultPollTime = 20 * time.Second

	// DefaultInitialVisibilityTimeout is the default initial message visibility timeout.
	DefaultInitialVisibilityTimeout = 5 * time.Second

	// DefaultMaxVisibilityTimeout is the default max message visibility timeout.
	DefaultMaxVisibilityTimeout = 60 * time.Second
)

var (
	// SQS message visibility timeout is a multiple of this.
	visTimeoutIncrement = time.Second

	// changeVisBuffer send request to change SQS message visibiilty timeout
	// this long before the timeout is actually reached.
	changeVisBuffer = 2 * time.Second

	// Randomize message visibility timeout by this percent to prevent thundering herds.
	sendVisRandomizationFactor = 0.05
)

// Handler is an interface for handling SQS messages.
// It's passed into SQS.Run().
type Handler interface {
	Handle(context.Context, *sqs.Message) error
}

//go:generate mockery -name=Handler

type funcHandler struct {
	Fn func(context.Context, *sqs.Message) error
}

func (h *funcHandler) Handle(ctx context.Context, m *sqs.Message) error {
	return h.Fn(ctx, m)
}

// FuncHandler returns a simple Handler wrapper around a function.
func FuncHandler(fn func(context.Context, *sqs.Message) error) Handler {
	return &funcHandler{
		Fn: fn,
	}
}

// SQS is a dispatcher for AWS SQS messages, calling a handler function in a
// goroutine and keeping the message reserved until its finished.
type SQS struct {
	client   sqsiface.SQSAPI
	queueURL string

	// Maximum number of concurrent handle funcion calls to
	// make at once. Zero (the default) is no limit.
	MaxConcurrent int

	// Max seconds to wait for messages.
	PollTime time.Duration

	// Initial message visibility timeout.
	InitialVisibilityTimeout time.Duration

	// Max message visibility timeout.
	MaxVisibilityTimeout time.Duration
}

// New returns a new SQS.
func New(client sqsiface.SQSAPI, queueURL string) *SQS {
	return &SQS{
		client:                   client,
		queueURL:                 queueURL,
		PollTime:                 DefaultPollTime,
		InitialVisibilityTimeout: DefaultInitialVisibilityTimeout,
		MaxVisibilityTimeout:     DefaultMaxVisibilityTimeout,
	}
}

// Run consumes messages from the SQS queue and calls handleF as a goroutine for each.
//
// While handleF is running, the message will periodically have its visibility timeout updated
// to keep the message reserved. If handleF or any communication with AWS returns a error,
// the context will be canceled and the error returned. If handleF returns without error,
// the message will be deleted from SQS.
func (q *SQS) Run(ctx context.Context, h Handler) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errc := make(chan error)

	receiveTicker := backoff.NewTicker(backoff.NewConstantBackOff(q.PollTime))
	defer receiveTicker.Stop()
	startReceive := receiveTicker.C // Put the channel in a separate var so we can enable/disable by setting nil
	receiveResults := make(chan []*sqs.Message)

	pendingHandle := make(map[string]struct{}) // Keep tracking of number of messages pending handle func.
	handleResults := make(chan *sqs.Message)

	postponec := make(chan postponeNotification)
	postponeNotification := make(map[string]func()) // ReceiptHandle => postpone cancel function
	defer func() {
		for _, cancel := range postponeNotification {
			cancel()
		}
	}()

	for { // Event loop. Everything within the select cases MUST NOT block.
		select {
		case <-ctx.Done(): // Stop on context cancel.
			return ctx.Err()

		case err := <-errc: // Stop on error.
			if err != context.Canceled {
				// Ignore context.Canceled from postpone case
				return err
			}

		case <-startReceive: // Start SQS message receive.
			var n int
			if q.MaxConcurrent == 0 {
				n = maxMessages
			} else {
				n = q.MaxConcurrent - len(pendingHandle)
			}
			if n > maxMessages {
				n = maxMessages
			}
			if n <= 0 {
				panic("tried to receive less than one messages")
			}
			go forwardErr(ctx, q.receive(ctx, n, receiveResults), errc)
			startReceive = nil // Disable start receive case

		case messages := <-receiveResults: // End SQS message receive and start handler goroutines.
			for _, m := range messages {
				pendingHandle[*m.ReceiptHandle] = struct{}{}
				go forwardErr(ctx, q.handle(ctx, h, m, handleResults, postponec), errc)
			}
			if q.MaxConcurrent == 0 || len(pendingHandle) < q.MaxConcurrent {
				startReceive = receiveTicker.C // Enable start receive case
			}

		case c := <-postponec: // Handle func is taking a while. Extend the message visibility timeout.
			rh := *c.M.ReceiptHandle
			if _, ok := pendingHandle[rh]; !ok {
				continue // handle already finished
			}
			if cancelPreviousPostpone, ok := postponeNotification[rh]; ok {
				// Cancel any previous running postpone goroutine.
				cancelPreviousPostpone()
			}
			msgCtx, cancelPostpone := context.WithCancel(ctx)
			postponeNotification[rh] = cancelPostpone
			go forwardErr(ctx, q.changeVisibilityTimeout(msgCtx, c.M, time.Until(c.T)), errc)

		case m := <-handleResults: // Delete message when done.
			go forwardErr(ctx, q.delete(ctx, m), errc)
			rh := *m.ReceiptHandle
			delete(pendingHandle, rh)
			if cancelPreviousPostpone, ok := postponeNotification[rh]; ok {
				// Cancel any previous running postpone goroutine.
				cancelPreviousPostpone()
				delete(postponeNotification, rh)
			}
			if q.MaxConcurrent == 0 || len(pendingHandle) < q.MaxConcurrent {
				startReceive = receiveTicker.C // Enable start receive case
			}
		}
	}
}

// receive fetches upto n (allowed values 1 to 10) messages from the AWS SQS queue in a goroutine
// and sends a list of messages to the results chan. The list might be empty if
// q.PollTime is exceeded.
func (q *SQS) receive(ctx context.Context, n int, results chan<- []*sqs.Message) <-chan error {
	errc := make(chan error, 1)
	go func() {
		defer close(errc)
		resp, err := q.client.ReceiveMessageWithContext(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(q.queueURL),
			MaxNumberOfMessages: aws.Int64(int64(n)),
			VisibilityTimeout:   aws.Int64(int64(q.InitialVisibilityTimeout / visTimeoutIncrement)),
			WaitTimeSeconds:     aws.Int64(int64(q.PollTime / visTimeoutIncrement)),
		})
		if err != nil {
			errc <- err
			return
		}
		select {
		case <-ctx.Done(): // Don't block if context is canceled.
			errc <- ctx.Err()
		case results <- resp.Messages:
		}
	}()
	return errc
}

// handle calls handleF on the given m as a goroutine, and sends either the resulting error to errc or m to done.
// If handleF takes time, messages will be sent to the postpone chan indicating that the SQS message visibility
// timeout should be updated.
func (q *SQS) handle(ctx context.Context, h Handler, m *sqs.Message, done chan<- *sqs.Message, postpone chan<- postponeNotification) <-chan error {
	errc := make(chan error, 1)
	go func() {
		defer close(errc)

		result := make(chan error, 1)
		go func() {
			result <- h.Handle(ctx, m)
		}()

		// Increase message visibility timeout in exponentially increasing amounts.
		b := &backoff.ExponentialBackOff{
			InitialInterval:     q.InitialVisibilityTimeout,
			RandomizationFactor: sendVisRandomizationFactor,
			Multiplier:          backoff.DefaultMultiplier,
			MaxInterval:         q.MaxVisibilityTimeout,
			MaxElapsedTime:      0,
			Clock:               backoff.SystemClock,
		}
		b.Reset()
		d := q.InitialVisibilityTimeout.Truncate(visTimeoutIncrement)

		var localPostpone chan<- postponeNotification
		var postponeMsg postponeNotification
		var err error
		var localErrc chan<- error
		startPostpone := time.After(d - changeVisBuffer)
		var localDone chan<- *sqs.Message

		for { // Event loop. Everything within the select cases MUST NOT block.
			select {
			case localDone <- m: // Finished successfully
				return
			case localErrc <- err: // Finished with error
				return
			case localPostpone <- postponeMsg: // Send the postpone message
				localPostpone = nil // Disable postpone send case
			case <-ctx.Done():
				err = ctx.Err()     // Set error return value
				localErrc = errc    // Enable error send case
				startPostpone = nil // Disable start postpone case
				localPostpone = nil // Disable send postpone case
			case err = <-result:
				if err != nil {
					localErrc = errc    // Enable error send case
					startPostpone = nil // Disable start postpone case
					localPostpone = nil // Disable send postpone case
				} else {
					localDone = done // Enable finished successfully
				}
			case t := <-startPostpone: // Schedule sending a postpone message
				// Enable send postpone message
				d = b.NextBackOff().Truncate(visTimeoutIncrement)
				postponeMsg = postponeNotification{
					M: m,
					T: t.Add(d + changeVisBuffer),
				}
				localPostpone = postpone

				// Schedule the next postpone
				startPostpone = time.After(d)
			}
		}
	}()
	return errc
}

// changeVisibilityTimeout changes the message visibility timeout in a goroutine to d (rounded
// up to the nearest visTimeoutIncrement).
func (q *SQS) changeVisibilityTimeout(ctx context.Context, m *sqs.Message, d time.Duration) <-chan error {
	if d < 0 {
		d = 0
	}
	errc := make(chan error, 1)
	go func() {
		defer close(errc)
		_, err := q.client.ChangeMessageVisibilityWithContext(ctx, &sqs.ChangeMessageVisibilityInput{
			QueueUrl:          aws.String(q.queueURL),
			ReceiptHandle:     m.ReceiptHandle,
			VisibilityTimeout: aws.Int64(int64((d + d%visTimeoutIncrement) / visTimeoutIncrement)),
		})
		if err != nil {
			errc <- err
		}
	}()
	return errc
}

// delete deletes the given message in a goroutine.
func (q *SQS) delete(ctx context.Context, m *sqs.Message) <-chan error {
	errc := make(chan error, 1)
	go func() {
		defer close(errc)
		_, err := q.client.DeleteMessageWithContext(ctx, &sqs.DeleteMessageInput{
			QueueUrl:      aws.String(q.queueURL),
			ReceiptHandle: m.ReceiptHandle,
		})
		if err != nil {
			errc <- err
		}
	}()
	return errc
}

type postponeNotification struct {
	M *sqs.Message // Change the visibility timeout of this message to....
	T time.Time    // this time.
}

// forwardErr selects errors from one channel and sends them to another.
// context.Canceled errors will be filtered out.
// It continues until the input channel closes or the context is canceled.
// If the context is canceled, it will keep trying to forward errors until one
// of the channels blocks, then exit.
func forwardErr(ctx context.Context, in <-chan error, out chan<- error) {
	var err error
	var ok bool
	localIn := in
	var localOut chan<- error
OuterLoop:
	for { // Event loop. Everything within the select cases MUST NOT block.

		// Prioritization hack: do this non-blocking select first. If one of cases succeeds immediately,
		// great, continue trying to forward errors. If any case blocks, go to the second select, where
		// we'll exit if the context has been canceled.
		select {
		case err, ok = <-localIn:
			if !ok {
				return
			}
			if err == context.Canceled {
				err = nil
			} else if err != nil {
				localIn = nil  // Disable input case
				localOut = out // Enable output case
			}
			continue OuterLoop
		case localOut <- err:
			localIn = in   // Enable input case
			localOut = nil // Disable output case
			continue OuterLoop
		default:
		}

		select {
		case err, ok = <-localIn:
			if !ok {
				return
			}
			if err == context.Canceled {
				err = nil
			} else if err != nil {
				localIn = nil  // Disable input case
				localOut = out // Enable output case
			}
		case localOut <- err:
			localIn = in   // Enable input case
			localOut = nil // Disable output case
		case <-ctx.Done():
			return
		}
	}
}
