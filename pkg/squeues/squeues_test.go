package squeues

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
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
)

const (
	queueURL = "https://sqs.us-east-2.amazonaws.com/123456789012/MyQueue"
	msgBody  = "i am a message body"
)

var nilReqOpts []request.Option

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

func setup(t *testing.T) (*SQS, *mockSQSClient, context.Context, func(), func()) {
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

	DefaultInitialVisibilityTimeout = 10 * time.Millisecond
	DefaultMaxVisibilityTimeout = 60 * time.Millisecond
	changeVisBuffer = 0
	visTimeoutIncrement = time.Millisecond
	DefaultPollTime = 20 * time.Millisecond
	sendVisRandomizationFactor = 0

	m := &mockSQSClient{}
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

type mockSQSClient struct {
	sqsiface.SQSAPI
	mock.Mock
}

func (m *mockSQSClient) ReceiveMessageWithContext(ctx aws.Context, input *sqs.ReceiveMessageInput, opts ...request.Option) (*sqs.ReceiveMessageOutput, error) {
	args := m.Called(ctx, input, opts)
	return args.Get(0).(*sqs.ReceiveMessageOutput), args.Error(1)
}

func (m *mockSQSClient) ChangeMessageVisibilityWithContext(ctx aws.Context, input *sqs.ChangeMessageVisibilityInput, opts ...request.Option) (*sqs.ChangeMessageVisibilityOutput, error) {
	args := m.Called(ctx, input, opts)
	return args.Get(0).(*sqs.ChangeMessageVisibilityOutput), args.Error(1)
}

func (m *mockSQSClient) DeleteMessageWithContext(ctx aws.Context, input *sqs.DeleteMessageInput, opts ...request.Option) (*sqs.DeleteMessageOutput, error) {
	args := m.Called(ctx, input, opts)
	return args.Get(0).(*sqs.DeleteMessageOutput), args.Error(1)
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
	q, m, ctx, cancel, teardown := setup(t)
	defer teardown()

	msg1 := makeMsg()
	msg1Delay := q.InitialVisibilityTimeout + q.InitialVisibilityTimeout/2
	msg1VisInput := mock.MatchedBy(func(input *sqs.ChangeMessageVisibilityInput) bool {
		return assert.Equal(t, q.queueURL, *input.QueueUrl) &&
			assert.Equal(t, *msg1.ReceiptHandle, *input.ReceiptHandle) &&
			assert.InDelta(t, q.InitialVisibilityTimeout/visTimeoutIncrement, *input.VisibilityTimeout, float64(visTimeoutIncrement))
	})
	m.On("ChangeMessageVisibilityWithContext", anyCtx, msg1VisInput, nilReqOpts).Once().Return(&sqs.ChangeMessageVisibilityOutput{}, error(nil))
	msg1DelInput := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(q.queueURL),
		ReceiptHandle: msg1.ReceiptHandle,
	}
	m.On("DeleteMessageWithContext", anyCtx, msg1DelInput, nilReqOpts).Once().Return(&sqs.DeleteMessageOutput{}, error(nil))

	msg2 := makeMsg()
	msg2DelInput := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(q.queueURL),
		ReceiptHandle: msg2.ReceiptHandle,
	}
	m.On("DeleteMessageWithContext", anyCtx, msg2DelInput, nilReqOpts).Once().Return(&sqs.DeleteMessageOutput{}, error(nil))

	expectedReceiveInput := &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(q.queueURL),
		MaxNumberOfMessages: aws.Int64(maxMessages),
		VisibilityTimeout:   aws.Int64(int64(q.InitialVisibilityTimeout / visTimeoutIncrement)),
		WaitTimeSeconds:     aws.Int64(int64(q.PollTime / visTimeoutIncrement)),
	}
	m.On("ReceiveMessageWithContext", anyCtx, expectedReceiveInput, nilReqOpts).After(q.PollTime/2).Once().Return(&sqs.ReceiveMessageOutput{Messages: []*sqs.Message{msg1, msg2}}, error(nil))
	// Additional receives return no messages
	m.On("ReceiveMessageWithContext", anyCtx, expectedReceiveInput, nilReqOpts).After(q.PollTime).Return(&sqs.ReceiveMessageOutput{Messages: []*sqs.Message{}}, error(nil))

	handleF := func(ctx context.Context, msg *sqs.Message) error {
		args := m.MethodCalled("handleF", ctx, msg)
		if err := ctx.Err(); err != nil {
			return err
		}
		return args.Error(0)
	}
	m.On("handleF", anyCtx, msg1).After(msg1Delay).Once().Return(error(nil))
	m.On("handleF", anyCtx, msg2).Once().Return(error(nil))

	shouldFinishIn := q.PollTime/2 + msg1Delay + 10*time.Millisecond
	doCancel := time.AfterFunc(shouldFinishIn, cancel)
	timeout := setTimeout(t, shouldFinishIn+10*time.Millisecond)
	err := q.Run(ctx, handleF)
	timeout.Stop()

	assert.False(t, doCancel.Stop(), "stopped before cancel")
	assert.Equal(t, context.Canceled, err)
	m.AssertCalled(t, "handleF", anyCtx, msg1)
	m.AssertCalled(t, "handleF", anyCtx, msg2)
	m.AssertCalled(t, "ReceiveMessageWithContext", anyCtx, expectedReceiveInput, nilReqOpts)
	m.AssertCalled(t, "ChangeMessageVisibilityWithContext", anyCtx, msg1VisInput, nilReqOpts)
	m.AssertCalled(t, "DeleteMessageWithContext", anyCtx, msg1DelInput, nilReqOpts)
	m.AssertCalled(t, "DeleteMessageWithContext", anyCtx, msg2DelInput, nilReqOpts)
}

func TestSQS_receive_success(t *testing.T) {
	q, m, ctx, _, teardown := setup(t)
	defer teardown()

	msg := makeMsg()
	m.On("ReceiveMessageWithContext",
		ctx,
		&sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(q.queueURL),
			MaxNumberOfMessages: aws.Int64(1),
			VisibilityTimeout:   aws.Int64(int64(q.InitialVisibilityTimeout / visTimeoutIncrement)),
			WaitTimeSeconds:     aws.Int64(int64(q.PollTime / visTimeoutIncrement)),
		},
		nilReqOpts,
	).Return(
		&sqs.ReceiveMessageOutput{
			Messages: []*sqs.Message{msg},
		},
		error(nil),
	)
	results := make(chan []*sqs.Message, 1)
	errc := q.receive(ctx, 1, results)
	assert.NoError(t, <-errc)
	close(results)
	r := <-results
	if assert.Len(t, r, 1) {
		assert.Equal(t, msg, r[0])
	}
	m.AssertCalled(t, "ReceiveMessageWithContext",
		anyCtx,
		&sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(q.queueURL),
			MaxNumberOfMessages: aws.Int64(1),
			VisibilityTimeout:   aws.Int64(int64(q.InitialVisibilityTimeout / visTimeoutIncrement)),
			WaitTimeSeconds:     aws.Int64(int64(q.PollTime / visTimeoutIncrement)),
		},
		nilReqOpts)
}

func TestSQS_receive_failure(t *testing.T) {
	q, m, ctx, _, teardown := setup(t)
	defer teardown()

	want := errors.New("test error")
	m.On("ReceiveMessageWithContext",
		ctx,
		&sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(q.queueURL),
			MaxNumberOfMessages: aws.Int64(1),
			VisibilityTimeout:   aws.Int64(int64(q.InitialVisibilityTimeout / visTimeoutIncrement)),
			WaitTimeSeconds:     aws.Int64(int64(q.PollTime / visTimeoutIncrement)),
		},
		nilReqOpts,
	).Return(
		(*sqs.ReceiveMessageOutput)(nil),
		want,
	)
	results := make(chan []*sqs.Message, 1)
	errc := q.receive(ctx, 1, results)
	assert.Equal(t, <-errc, want)
	close(results)
	assert.Zero(t, <-results)
	m.AssertCalled(t, "ReceiveMessageWithContext",
		anyCtx,
		&sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(q.queueURL),
			MaxNumberOfMessages: aws.Int64(1),
			VisibilityTimeout:   aws.Int64(int64(q.InitialVisibilityTimeout / visTimeoutIncrement)),
			WaitTimeSeconds:     aws.Int64(int64(q.PollTime / visTimeoutIncrement)),
		},
		nilReqOpts)
}

// Test context cancel prevents goroutine from blocking
func TestSQS_receive_ctxCancel(t *testing.T) {
	q, m, ctx, cancel, teardown := setup(t)
	defer teardown()

	finishIn := q.InitialVisibilityTimeout / 2
	m.On("ReceiveMessageWithContext",
		anyCtx,
		&sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(q.queueURL),
			MaxNumberOfMessages: aws.Int64(1),
			VisibilityTimeout:   aws.Int64(int64(q.InitialVisibilityTimeout / visTimeoutIncrement)),
			WaitTimeSeconds:     aws.Int64(int64(q.PollTime / visTimeoutIncrement)),
		},
		nilReqOpts,
	).After(finishIn).Return((*sqs.ReceiveMessageOutput)(nil), context.Canceled)
	results := make(chan []*sqs.Message)

	doCancel := time.AfterFunc(finishIn/2, cancel)
	timeout := setTimeout(t, finishIn+10*time.Millisecond)
	err := <-q.receive(ctx, 1, results)
	timeout.Stop()
	assert.False(t, doCancel.Stop(), "stopped before cancel")
	assert.Equal(t, context.Canceled, err)
	close(results)
	assert.Zero(t, <-results)
	m.AssertCalled(t, "ReceiveMessageWithContext",
		anyCtx,
		&sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(q.queueURL),
			MaxNumberOfMessages: aws.Int64(1),
			VisibilityTimeout:   aws.Int64(int64(q.InitialVisibilityTimeout / visTimeoutIncrement)),
			WaitTimeSeconds:     aws.Int64(int64(q.PollTime / visTimeoutIncrement)),
		},
		nilReqOpts)
}

func TestSQS_handle_success(t *testing.T) {
	q, _, ctx, _, teardown := setup(t)
	defer teardown()

	m := &mock.Mock{}
	msg := makeMsg()
	postpone := make(chan postponeNotification, 1)
	done := make(chan *sqs.Message, 1)
	handleF := func(ctx context.Context, msg *sqs.Message) error {
		args := m.MethodCalled("handleF", ctx, msg)
		return args.Error(0)
	}

	m.On("handleF", anyCtx, msg).Return(error(nil))
	errc := q.handle(ctx, handleF, msg, done, postpone)
	assert.NoError(t, <-errc)
	close(postpone)
	close(done)
	assert.Zero(t, <-postpone)
	assert.Equal(t, msg, <-done)
	m.AssertCalled(t, "handleF", anyCtx, msg)
}
func TestSQS_handle_postpone(t *testing.T) {
	q, _, ctx, _, teardown := setup(t)
	defer teardown()

	m := &mock.Mock{}
	msg := makeMsg()
	postpone := make(chan postponeNotification, 1)
	done := make(chan *sqs.Message, 1)
	handleF := func(ctx context.Context, msg *sqs.Message) error {
		args := m.MethodCalled("handleF", ctx, msg)
		return args.Error(0)
	}

	finishIn := q.InitialVisibilityTimeout + visTimeoutIncrement
	m.On("handleF", anyCtx, msg).After(finishIn).Return(error(nil))

	now := time.Now()
	timeout := setTimeout(t, finishIn+10*time.Millisecond)
	err := <-q.handle(ctx, handleF, msg, done, postpone)
	timeout.Stop()
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
	m.AssertCalled(t, "handleF", anyCtx, msg)
}

func TestSQS_handle_failure(t *testing.T) {
	q, _, ctx, _, teardown := setup(t)
	defer teardown()

	m := &mock.Mock{}
	msg := makeMsg()
	postpone := make(chan postponeNotification, 1)
	done := make(chan *sqs.Message, 1)
	handleF := func(ctx context.Context, msg *sqs.Message) error {
		args := m.MethodCalled("handleF", ctx, msg)
		return args.Error(0)
	}

	want := errors.New("test error")
	m.On("handleF", anyCtx, msg).Return(want)
	errc := q.handle(ctx, handleF, msg, done, postpone)
	assert.Equal(t, want, <-errc)
	close(postpone)
	assert.Zero(t, <-postpone)
	close(done)
	assert.Zero(t, <-done)
	m.AssertCalled(t, "handleF", anyCtx, msg)
}

// Test context cancel prevents goroutine from blocking
func TestSQS_handle_ctxCancel(t *testing.T) {
	q, _, ctx, cancel, teardown := setup(t)
	defer teardown()

	m := &mock.Mock{}
	msg := makeMsg()
	postpone := make(chan postponeNotification)
	done := make(chan *sqs.Message)
	handleF := func(ctx context.Context, msg *sqs.Message) error {
		args := m.MethodCalled("handleF", ctx, msg)
		return args.Error(0)
	}

	finishIn := q.InitialVisibilityTimeout / 2
	m.On("handleF", anyCtx, msg).After(finishIn).Return(error(nil))

	doCancel := time.AfterFunc(finishIn/2, cancel)
	timeout := setTimeout(t, finishIn+10*time.Millisecond)
	err := <-q.handle(ctx, handleF, msg, done, postpone)
	timeout.Stop()

	assert.False(t, doCancel.Stop(), "stopped before cancel")
	assert.Equal(t, context.Canceled, err)
	close(postpone)
	assert.Zero(t, <-postpone)
	close(done)
	assert.Zero(t, <-done)
	m.AssertCalled(t, "handleF", anyCtx, msg)
}

func TestSQS_changeVisibilityTimeout_success(t *testing.T) {
	q, m, ctx, _, teardown := setup(t)
	defer teardown()

	msg := makeMsg()
	d := 17 * time.Second

	m.On("ChangeMessageVisibilityWithContext",
		anyCtx,
		&sqs.ChangeMessageVisibilityInput{
			QueueUrl:          aws.String(q.queueURL),
			ReceiptHandle:     msg.ReceiptHandle,
			VisibilityTimeout: aws.Int64(int64(d / visTimeoutIncrement)),
		},
		nilReqOpts,
	).Return(
		&sqs.ChangeMessageVisibilityOutput{},
		error(nil),
	)
	errc := q.changeVisibilityTimeout(ctx, msg, d)
	assert.NoError(t, <-errc)
	m.AssertCalled(t, "ChangeMessageVisibilityWithContext",
		anyCtx,
		&sqs.ChangeMessageVisibilityInput{
			QueueUrl:          aws.String(q.queueURL),
			ReceiptHandle:     msg.ReceiptHandle,
			VisibilityTimeout: aws.Int64(int64(d / visTimeoutIncrement)),
		},
		nilReqOpts,
	)
}
func TestSQS_changeVisibilityTimeout_failure(t *testing.T) {
	q, m, ctx, _, teardown := setup(t)
	defer teardown()

	msg := makeMsg()
	d := 17 * time.Second

	want := errors.New("test error")
	m.On("ChangeMessageVisibilityWithContext",
		anyCtx,
		&sqs.ChangeMessageVisibilityInput{
			QueueUrl:          aws.String(q.queueURL),
			ReceiptHandle:     msg.ReceiptHandle,
			VisibilityTimeout: aws.Int64(int64(d / visTimeoutIncrement)),
		},
		nilReqOpts,
	).Return(
		(*sqs.ChangeMessageVisibilityOutput)(nil),
		want,
	)
	errc := q.changeVisibilityTimeout(ctx, msg, d)
	assert.Equal(t, want, <-errc)
	m.AssertCalled(t, "ChangeMessageVisibilityWithContext",
		anyCtx,
		&sqs.ChangeMessageVisibilityInput{
			QueueUrl:          aws.String(q.queueURL),
			ReceiptHandle:     msg.ReceiptHandle,
			VisibilityTimeout: aws.Int64(int64(d / visTimeoutIncrement)),
		},
		nilReqOpts,
	)
}

// Test context cancel prevents goroutine from blocking
func TestSQS_changeVisibilityTimeout_ctxCancel(t *testing.T) {
	q, m, ctx, cancel, teardown := setup(t)
	defer teardown()

	msg := makeMsg()
	d := 17 * time.Second

	finishIn := q.InitialVisibilityTimeout / 2
	m.On("ChangeMessageVisibilityWithContext",
		anyCtx,
		&sqs.ChangeMessageVisibilityInput{
			QueueUrl:          aws.String(q.queueURL),
			ReceiptHandle:     msg.ReceiptHandle,
			VisibilityTimeout: aws.Int64(int64(d / visTimeoutIncrement)),
		},
		nilReqOpts,
	).After(finishIn).Return((*sqs.ChangeMessageVisibilityOutput)(nil), context.Canceled)

	doCancel := time.AfterFunc(finishIn/2, cancel)
	timeout := setTimeout(t, finishIn+10*time.Millisecond)
	err := <-q.changeVisibilityTimeout(ctx, msg, d)
	timeout.Stop()

	assert.False(t, doCancel.Stop(), "stopped before cancel")
	assert.Equal(t, context.Canceled, err)
	m.AssertCalled(t, "ChangeMessageVisibilityWithContext",
		anyCtx,
		&sqs.ChangeMessageVisibilityInput{
			QueueUrl:          aws.String(q.queueURL),
			ReceiptHandle:     msg.ReceiptHandle,
			VisibilityTimeout: aws.Int64(int64(d / visTimeoutIncrement)),
		},
		nilReqOpts,
	)
}

func TestSQS_delete_success(t *testing.T) {
	q, m, ctx, _, teardown := setup(t)
	defer teardown()

	msg := makeMsg()

	m.On("DeleteMessageWithContext",
		anyCtx,
		&sqs.DeleteMessageInput{
			QueueUrl:      aws.String(q.queueURL),
			ReceiptHandle: msg.ReceiptHandle,
		},
		nilReqOpts,
	).Return(
		&sqs.DeleteMessageOutput{},
		error(nil),
	)
	errc := q.delete(ctx, msg)
	assert.NoError(t, <-errc)
	m.AssertCalled(t, "DeleteMessageWithContext",
		anyCtx,
		&sqs.DeleteMessageInput{
			QueueUrl:      aws.String(q.queueURL),
			ReceiptHandle: msg.ReceiptHandle,
		},
		nilReqOpts,
	)
}
func TestSQS_delete_failure(t *testing.T) {
	q, m, ctx, _, teardown := setup(t)
	defer teardown()

	msg := makeMsg()

	want := errors.New("test error")
	m.On("DeleteMessageWithContext",
		anyCtx,
		&sqs.DeleteMessageInput{
			QueueUrl:      aws.String(q.queueURL),
			ReceiptHandle: msg.ReceiptHandle,
		},
		nilReqOpts,
	).Return((*sqs.DeleteMessageOutput)(nil), want)
	errc := q.delete(ctx, msg)
	assert.Equal(t, want, <-errc)
	m.AssertCalled(t, "DeleteMessageWithContext",
		anyCtx,
		&sqs.DeleteMessageInput{
			QueueUrl:      aws.String(q.queueURL),
			ReceiptHandle: msg.ReceiptHandle,
		},
		nilReqOpts,
	)
}

// Test context cancel prevents goroutine from blocking
func TestSQS_delete_ctxCancel(t *testing.T) {
	q, m, ctx, cancel, teardown := setup(t)
	defer teardown()

	msg := makeMsg()

	finishIn := q.InitialVisibilityTimeout / 2
	m.On("DeleteMessageWithContext",
		anyCtx,
		&sqs.DeleteMessageInput{
			QueueUrl:      aws.String(q.queueURL),
			ReceiptHandle: msg.ReceiptHandle,
		},
		nilReqOpts,
	).After(finishIn).Return((*sqs.DeleteMessageOutput)(nil), context.Canceled)

	doCancel := time.AfterFunc(finishIn/2, cancel)
	timeout := setTimeout(t, finishIn+10*time.Millisecond)
	err := <-q.delete(ctx, msg)
	timeout.Stop()

	assert.False(t, doCancel.Stop(), "stopped before cancel")
	assert.Equal(t, context.Canceled, err)
	m.AssertCalled(t, "DeleteMessageWithContext",
		anyCtx,
		&sqs.DeleteMessageInput{
			QueueUrl:      aws.String(q.queueURL),
			ReceiptHandle: msg.ReceiptHandle,
		},
		nilReqOpts,
	)
}

func setTimeout(t *testing.T, d time.Duration) *time.Timer {
	return time.AfterFunc(d, func() {
		t.Fatal("timed out")
	})
}

var anyCtx = mock.MatchedBy(func(ctx context.Context) bool {
	return true
})
