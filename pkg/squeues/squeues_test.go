package squeues

//go:generate mockery -name=Handler
//go:generate sh -c "mockery -name=SQSAPI -dir=$(go list -f '{{.Dir}}' github.com/aws/aws-sdk-go/service/sqs/sqsiface)"

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"

	"github.com/mintel/elasticsearch-asg/pkg/squeues/mocks"
)

const (
	queueURL = "https://sqs.us-east-2.amazonaws.com/123456789012/MyQueue"
	msgBody  = "i am a message body"
)

func makeMsg() *sqs.Message {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	messageID := base64.URLEncoding.EncodeToString(b)
	h := md5.Sum([]byte(msgBody))
	receiptHandle := uuid.New().String()
	return &sqs.Message{
		MessageId:     aws.String(messageID),
		MD5OfBody:     aws.String(string(h[:])),
		Body:          aws.String(msgBody),
		ReceiptHandle: aws.String(receiptHandle),
	}
}

func setup(t *testing.T) (*SQS, *mocks.SQSAPI, context.Context, func(), func()) {
	logger := zaptest.NewLogger(t)
	f1 := zap.ReplaceGlobals(logger)
	f2 := zap.RedirectStdLog(logger)

	ctx, cancel := context.WithCancel(context.Background())

	originalDefaultInitialVisibilityTimeout := DefaultInitialVisibilityTimeout
	originalDefaultMaxVisibilityTimeout := DefaultMaxVisibilityTimeout
	originalChangeVisBuffer := changeVisBuffer
	originalVisTimeoutIncrement := visTimeoutIncrement
	originalDefaultPollTime := DefaultPollTime
	originalSendVisRandomizationFactor := sendVisRandomizationFactor

	DefaultInitialVisibilityTimeout = 50 * time.Millisecond
	DefaultMaxVisibilityTimeout = 300 * time.Millisecond
	changeVisBuffer = 0
	visTimeoutIncrement = time.Millisecond
	DefaultPollTime = 20 * time.Millisecond
	sendVisRandomizationFactor = 0

	m := &mocks.SQSAPI{}
	m.Test(t)

	q := New(m, queueURL)

	teardown := func() {
		cancel()
		f2()
		f1()
		DefaultInitialVisibilityTimeout = originalDefaultInitialVisibilityTimeout
		DefaultMaxVisibilityTimeout = originalDefaultMaxVisibilityTimeout
		changeVisBuffer = originalChangeVisBuffer
		visTimeoutIncrement = originalVisTimeoutIncrement
		DefaultPollTime = originalDefaultPollTime
		sendVisRandomizationFactor = originalSendVisRandomizationFactor
		if err := logger.Sync(); err != nil {
			panic(err)
		}
	}
	return q, m, ctx, cancel, teardown
}

func TestSQS_New(t *testing.T) {
	q, _, _, _, teardown := setup(t)
	defer teardown()
	assert.Equal(t, 0, q.MaxConcurrent)
	assert.Equal(t, DefaultPollTime, q.PollTime)
	assert.Equal(t, DefaultInitialVisibilityTimeout, q.InitialVisibilityTimeout)
	assert.Equal(t, DefaultMaxVisibilityTimeout, q.MaxVisibilityTimeout)
}

func TestSQS_Run(t *testing.T) {
	q, sqsSvc, ctx, cancel, teardown := setup(t)
	defer teardown()

	msg1 := makeMsg()
	msg1Delay := q.InitialVisibilityTimeout + q.InitialVisibilityTimeout/2
	msg1VisInput := mock.MatchedBy(func(input *sqs.ChangeMessageVisibilityInput) bool {
		return assert.Equal(t, q.queueURL, *input.QueueUrl) &&
			assert.Equal(t, *msg1.ReceiptHandle, *input.ReceiptHandle) &&
			assert.InDelta(t, q.InitialVisibilityTimeout/visTimeoutIncrement, *input.VisibilityTimeout, float64(visTimeoutIncrement))
	})
	sqsSvc.On("ChangeMessageVisibilityWithContext", AnyContext, msg1VisInput).Once().Return(&sqs.ChangeMessageVisibilityOutput{}, error(nil))
	msg1DelInput := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(q.queueURL),
		ReceiptHandle: msg1.ReceiptHandle,
	}
	sqsSvc.On("DeleteMessageWithContext", AnyContext, msg1DelInput).Once().Return(&sqs.DeleteMessageOutput{}, error(nil))

	msg2 := makeMsg()
	msg2DelInput := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(q.queueURL),
		ReceiptHandle: msg2.ReceiptHandle,
	}
	sqsSvc.On("DeleteMessageWithContext", AnyContext, msg2DelInput).Once().Return(&sqs.DeleteMessageOutput{}, error(nil))

	expectedReceiveInput := &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(q.queueURL),
		MaxNumberOfMessages: aws.Int64(maxMessages),
		VisibilityTimeout:   aws.Int64(int64(q.InitialVisibilityTimeout / visTimeoutIncrement)),
		WaitTimeSeconds:     aws.Int64(int64(q.PollTime / visTimeoutIncrement)),
	}
	sqsSvc.On("ReceiveMessageWithContext", AnyContext, expectedReceiveInput).After(q.PollTime/2).Once().Return(&sqs.ReceiveMessageOutput{Messages: []*sqs.Message{msg1, msg2}}, error(nil))
	// Additional receives return no messages
	sqsSvc.On("ReceiveMessageWithContext", AnyContext, expectedReceiveInput).After(q.PollTime).Return(&sqs.ReceiveMessageOutput{Messages: []*sqs.Message{}}, error(nil))

	handler := &mocks.Handler{}
	handler.On("Handle", AnyContext, msg1).After(msg1Delay).Once().Return(error(nil))
	handler.On("Handle", AnyContext, msg2).Once().Return(error(nil))

	shouldFinishIn := q.PollTime/2 + msg1Delay + 10*time.Millisecond
	doCancel := time.AfterFunc(shouldFinishIn, cancel)
	err := q.Run(ctx, handler)

	assert.False(t, doCancel.Stop(), "stopped before cancel")
	assert.Equal(t, context.Canceled, err)
	sqsSvc.AssertExpectations(t)
	handler.AssertExpectations(t)
}

func TestSQS_receive_success(t *testing.T) {
	q, sqsSvc, ctx, _, teardown := setup(t)
	defer teardown()

	msg := makeMsg()
	sqsSvc.On("ReceiveMessageWithContext",
		ctx,
		&sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(q.queueURL),
			MaxNumberOfMessages: aws.Int64(1),
			VisibilityTimeout:   aws.Int64(int64(q.InitialVisibilityTimeout / visTimeoutIncrement)),
			WaitTimeSeconds:     aws.Int64(int64(q.PollTime / visTimeoutIncrement)),
		},
	).Return(&sqs.ReceiveMessageOutput{Messages: []*sqs.Message{msg}}, error(nil))

	results := make(chan []*sqs.Message, 1)
	errc := q.receive(ctx, 1, results)
	assert.NoError(t, <-errc)
	close(results)
	r := <-results
	if assert.Len(t, r, 1) {
		assert.Equal(t, msg, r[0])
	}
	sqsSvc.AssertExpectations(t)
}

func TestSQS_receive_failure(t *testing.T) {
	q, sqsSvc, ctx, _, teardown := setup(t)
	defer teardown()

	want := errors.New("test error")
	sqsSvc.On("ReceiveMessageWithContext",
		ctx,
		&sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(q.queueURL),
			MaxNumberOfMessages: aws.Int64(1),
			VisibilityTimeout:   aws.Int64(int64(q.InitialVisibilityTimeout / visTimeoutIncrement)),
			WaitTimeSeconds:     aws.Int64(int64(q.PollTime / visTimeoutIncrement)),
		},
	).Return((*sqs.ReceiveMessageOutput)(nil), want)
	results := make(chan []*sqs.Message, 1)
	errc := q.receive(ctx, 1, results)
	assert.Equal(t, <-errc, want)
	close(results)
	assert.Zero(t, <-results)
	sqsSvc.AssertExpectations(t)
}

// Test context cancel prevents goroutine from blocking
func TestSQS_receive_ctxCancel(t *testing.T) {
	q, sqsSvc, ctx, cancel, teardown := setup(t)
	defer teardown()

	finishIn := q.InitialVisibilityTimeout / 2
	sqsSvc.On("ReceiveMessageWithContext",
		AnyContext,
		&sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(q.queueURL),
			MaxNumberOfMessages: aws.Int64(1),
			VisibilityTimeout:   aws.Int64(int64(q.InitialVisibilityTimeout / visTimeoutIncrement)),
			WaitTimeSeconds:     aws.Int64(int64(q.PollTime / visTimeoutIncrement)),
		},
	).After(finishIn).Return((*sqs.ReceiveMessageOutput)(nil), context.Canceled)
	results := make(chan []*sqs.Message)

	doCancel := time.AfterFunc(finishIn/2, cancel)
	err := <-q.receive(ctx, 1, results)
	assert.False(t, doCancel.Stop(), "stopped before cancel")
	assert.Equal(t, context.Canceled, err)
	close(results)
	assert.Zero(t, <-results)
	sqsSvc.AssertExpectations(t)
}

func TestSQS_handle_success(t *testing.T) {
	q, _, ctx, _, teardown := setup(t)
	defer teardown()

	m := &mock.Mock{}
	msg := makeMsg()
	postpone := make(chan postponeNotification, 1)
	done := make(chan *sqs.Message, 1)

	handler := &mocks.Handler{}
	handler.On("Handle", AnyContext, msg).Return(error(nil))

	errc := q.handle(ctx, handler, msg, done, postpone)
	assert.NoError(t, <-errc)
	close(postpone)
	close(done)
	assert.Zero(t, <-postpone)
	assert.Equal(t, msg, <-done)
	m.AssertExpectations(t)
	handler.AssertExpectations(t)
}
func TestSQS_handle_postpone(t *testing.T) {
	q, _, ctx, _, teardown := setup(t)
	defer teardown()

	msg := makeMsg()
	postpone := make(chan postponeNotification, 1)
	done := make(chan *sqs.Message, 1)

	finishIn := q.InitialVisibilityTimeout + visTimeoutIncrement
	handler := &mocks.Handler{}
	handler.On("Handle", AnyContext, msg).After(finishIn).Return(error(nil))

	now := time.Now()
	err := <-q.handle(ctx, handler, msg, done, postpone)
	assert.NoError(t, err)

	assert.NoError(t, err)
	got := <-postpone
	if assert.NotZero(t, got) {
		assert.Equal(t, msg, got.M)
		want := now.Add(2 * q.InitialVisibilityTimeout)
		delta := time.Duration(float64(q.InitialVisibilityTimeout)*sendVisRandomizationFactor) + time.Millisecond
		assert.WithinDuration(t, want, got.T, delta)
	}
	assert.Equal(t, msg, <-done)
	handler.AssertExpectations(t)
}

func TestSQS_handle_failure(t *testing.T) {
	q, _, ctx, _, teardown := setup(t)
	defer teardown()

	msg := makeMsg()
	postpone := make(chan postponeNotification, 1)
	done := make(chan *sqs.Message, 1)

	want := errors.New("test error")
	handler := &mocks.Handler{}
	handler.On("Handle", AnyContext, msg).Return(want)

	errc := q.handle(ctx, handler, msg, done, postpone)
	assert.Equal(t, want, <-errc)
	close(postpone)
	assert.Zero(t, <-postpone)
	close(done)
	assert.Zero(t, <-done)
	handler.AssertExpectations(t)
}

// Test context cancel prevents goroutine from blocking
func TestSQS_handle_ctxCancel(t *testing.T) {
	q, _, ctx, cancel, teardown := setup(t)
	defer teardown()

	msg := makeMsg()
	postpone := make(chan postponeNotification)
	done := make(chan *sqs.Message)

	finishIn := q.InitialVisibilityTimeout / 2
	handler := &mocks.Handler{}
	handler.On("Handle", AnyContext, msg).After(finishIn).Return(error(nil))

	doCancel := time.AfterFunc(finishIn/2, cancel)
	err := <-q.handle(ctx, handler, msg, done, postpone)

	assert.False(t, doCancel.Stop(), "stopped before cancel")
	assert.Equal(t, context.Canceled, err)
	close(postpone)
	assert.Zero(t, <-postpone)
	close(done)
	assert.Zero(t, <-done)
	handler.AssertExpectations(t)
}

func TestSQS_changeVisibilityTimeout_success(t *testing.T) {
	q, sqsSvc, ctx, _, teardown := setup(t)
	defer teardown()

	msg := makeMsg()
	d := 17 * time.Second

	sqsSvc.On("ChangeMessageVisibilityWithContext",
		AnyContext,
		&sqs.ChangeMessageVisibilityInput{
			QueueUrl:          aws.String(q.queueURL),
			ReceiptHandle:     msg.ReceiptHandle,
			VisibilityTimeout: aws.Int64(int64(d / visTimeoutIncrement)),
		},
	).Return(
		&sqs.ChangeMessageVisibilityOutput{},
		error(nil),
	)
	errc := q.changeVisibilityTimeout(ctx, msg, d)
	assert.NoError(t, <-errc)
	sqsSvc.AssertExpectations(t)
}
func TestSQS_changeVisibilityTimeout_failure(t *testing.T) {
	q, sqsSvc, ctx, _, teardown := setup(t)
	defer teardown()

	msg := makeMsg()
	d := 17 * time.Second

	want := errors.New("test error")
	sqsSvc.On("ChangeMessageVisibilityWithContext",
		AnyContext,
		&sqs.ChangeMessageVisibilityInput{
			QueueUrl:          aws.String(q.queueURL),
			ReceiptHandle:     msg.ReceiptHandle,
			VisibilityTimeout: aws.Int64(int64(d / visTimeoutIncrement)),
		},
	).Return(
		(*sqs.ChangeMessageVisibilityOutput)(nil),
		want,
	)
	errc := q.changeVisibilityTimeout(ctx, msg, d)
	assert.Equal(t, want, <-errc)
	sqsSvc.AssertExpectations(t)
}

// Test context cancel prevents goroutine from blocking
func TestSQS_changeVisibilityTimeout_ctxCancel(t *testing.T) {
	q, sqsSvc, ctx, cancel, teardown := setup(t)
	defer teardown()

	msg := makeMsg()
	d := 17 * time.Second

	finishIn := q.InitialVisibilityTimeout / 2
	sqsSvc.On("ChangeMessageVisibilityWithContext",
		AnyContext,
		&sqs.ChangeMessageVisibilityInput{
			QueueUrl:          aws.String(q.queueURL),
			ReceiptHandle:     msg.ReceiptHandle,
			VisibilityTimeout: aws.Int64(int64(d / visTimeoutIncrement)),
		},
	).After(finishIn).Return((*sqs.ChangeMessageVisibilityOutput)(nil), context.Canceled)

	doCancel := time.AfterFunc(finishIn/2, cancel)
	err := <-q.changeVisibilityTimeout(ctx, msg, d)

	assert.False(t, doCancel.Stop(), "stopped before cancel")
	assert.Equal(t, context.Canceled, err)
	sqsSvc.AssertExpectations(t)
}

func TestSQS_delete_success(t *testing.T) {
	q, sqsSvc, ctx, _, teardown := setup(t)
	defer teardown()

	msg := makeMsg()

	sqsSvc.On("DeleteMessageWithContext",
		AnyContext,
		&sqs.DeleteMessageInput{
			QueueUrl:      aws.String(q.queueURL),
			ReceiptHandle: msg.ReceiptHandle,
		},
	).Return(
		&sqs.DeleteMessageOutput{},
		error(nil),
	)
	errc := q.delete(ctx, msg)
	assert.NoError(t, <-errc)
	sqsSvc.AssertExpectations(t)
}
func TestSQS_delete_failure(t *testing.T) {
	q, sqsSvc, ctx, _, teardown := setup(t)
	defer teardown()

	msg := makeMsg()

	want := errors.New("test error")
	sqsSvc.On("DeleteMessageWithContext",
		AnyContext,
		&sqs.DeleteMessageInput{
			QueueUrl:      aws.String(q.queueURL),
			ReceiptHandle: msg.ReceiptHandle,
		},
	).Return((*sqs.DeleteMessageOutput)(nil), want)
	errc := q.delete(ctx, msg)
	assert.Equal(t, want, <-errc)
	sqsSvc.AssertExpectations(t)
}

// Test context cancel prevents goroutine from blocking
func TestSQS_delete_ctxCancel(t *testing.T) {
	q, sqsSvc, ctx, cancel, teardown := setup(t)
	defer teardown()

	msg := makeMsg()

	finishIn := q.InitialVisibilityTimeout / 2
	sqsSvc.On("DeleteMessageWithContext",
		AnyContext,
		&sqs.DeleteMessageInput{
			QueueUrl:      aws.String(q.queueURL),
			ReceiptHandle: msg.ReceiptHandle,
		},
	).After(finishIn).Return((*sqs.DeleteMessageOutput)(nil), context.Canceled)

	doCancel := time.AfterFunc(finishIn/2, cancel)
	err := <-q.delete(ctx, msg)

	assert.False(t, doCancel.Stop(), "stopped before cancel")
	assert.Equal(t, context.Canceled, err)
	sqsSvc.AssertExpectations(t)
}

// AnyContext can be used in mock assertions to test that a context.Context was passed.
//
//   // func (*SQSAPI) ReceiveMessageWithContext(context.Context, *sqs.ReceiveMessageInput, ...request.Option) (*sqs.ReceiveMessageOutput, error)
//   m.On("ReceiveMessageWithContext", awsmock.AnyContext, input, awsmock.NilOpts)
var AnyContext = mock.MatchedBy(func(ctx context.Context) bool {
	return true
})
