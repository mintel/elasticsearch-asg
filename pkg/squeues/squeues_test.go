package squeues

//go:generate mockery -name=Handler
//go:generate sh -c "mockery -name=SQSAPI -dir=$(go list -f '{{.Dir}}' github.com/aws/aws-sdk-go/service/sqs/sqsiface)"

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"math"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/cenkalti/backoff"       // Backoff/retry utils
	"github.com/google/uuid"            // Generate UUIDs
	"github.com/stretchr/testify/mock"  // Mocking
	"github.com/stretchr/testify/suite" // Test suite for setup and teardown

	// AWS clients and stuff
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"

	"github.com/mintel/elasticsearch-asg/pkg/squeues/mocks" // Mocked interfaces
)

const (
	testQueueURL                        = "https://sqs.us-east-2.amazonaws.com/123456789012/MyQueue"
	testSecond                          = 25 * time.Millisecond
	testDefaultInitialVisibilityTimeout = 10 * testSecond
	testDefaultMaxVisibilityTimeout     = 25 * testSecond
	testAWSCommBuffer                   = 2 * testSecond
	testDefaultPollTime                 = 4 * testSecond
	testSendVisRandomizationFactor      = 0
)

// makeMsg makes a fake SQS message.
func makeMsg(name string) *sqs.Message {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	messageID := base64.URLEncoding.EncodeToString(b) + "-" + name
	h := md5.Sum([]byte(name))
	receiptHandle := uuid.New().String() + "-" + name
	return &sqs.Message{
		MessageId:     aws.String(messageID),
		MD5OfBody:     aws.String(base64.URLEncoding.EncodeToString(h[:])),
		Body:          aws.String(name),
		ReceiptHandle: aws.String(receiptHandle),
	}
}

// DispatcherTestSuite runs tests for Dispatcher,
// doing setup and teardown before and after each test.
type DispatcherTestSuite struct {
	suite.Suite

	originalDefaultInitialVisibilityTimeout time.Duration
	originalDefaultMaxVisibilityTimeout     time.Duration
	originalAWSCommBuffer                   time.Duration
	originalSecond                          time.Duration
	originalDefaultPollTime                 time.Duration
	originalSendVisRandomizationFactor      float64

	SUT     *Dispatcher // System Under Test
	MockSQS *mocks.SQSAPI
}

// SetupTest runs before each test.
func (suite *DispatcherTestSuite) SetupTest() {
	suite.originalDefaultInitialVisibilityTimeout = DefaultInitialVisibilityTimeout
	suite.originalDefaultMaxVisibilityTimeout = DefaultMaxVisibilityTimeout
	suite.originalAWSCommBuffer = awsCommBuffer
	suite.originalSecond = second
	suite.originalDefaultPollTime = DefaultPollTime
	suite.originalSendVisRandomizationFactor = sendVisRandomizationFactor

	// Set global package vars to smaller, more stable values.
	// Otherwise tests would take _way_ too long.
	DefaultInitialVisibilityTimeout = testDefaultInitialVisibilityTimeout
	DefaultMaxVisibilityTimeout = testDefaultMaxVisibilityTimeout
	awsCommBuffer = testAWSCommBuffer
	second = testSecond
	DefaultPollTime = testDefaultPollTime
	sendVisRandomizationFactor = testSendVisRandomizationFactor

	// Set up mock AWS SQS client.
	suite.MockSQS = &mocks.SQSAPI{}
	suite.MockSQS.Test(suite.T())

	// Set up Dispatcher.
	suite.SUT = New(suite.MockSQS, testQueueURL)
}

// TearDownTest runs after each test.
func (suite *DispatcherTestSuite) TearDownTest() {
	// Assert that the mock was called as expected.
	suite.MockSQS.AssertExpectations(suite.T())

	// Reset original global package vars.
	DefaultInitialVisibilityTimeout = suite.originalDefaultInitialVisibilityTimeout
	DefaultMaxVisibilityTimeout = suite.originalDefaultMaxVisibilityTimeout
	awsCommBuffer = suite.originalAWSCommBuffer
	second = suite.originalSecond
	DefaultPollTime = suite.originalDefaultPollTime
	sendVisRandomizationFactor = suite.originalSendVisRandomizationFactor
}

// TestNew tests new Dispatcher defaults.
func (suite *DispatcherTestSuite) TestNew() {
	suite.Equal(0, suite.SUT.MaxConcurrent)
	suite.Equal(testDefaultPollTime, suite.SUT.PollTime)
	suite.Equal(testDefaultInitialVisibilityTimeout, suite.SUT.InitialVisibilityTimeout)
	suite.Equal(testDefaultMaxVisibilityTimeout, suite.SUT.MaxVisibilityTimeout)
}

// TestRunWithContext tests the Dispatcher.RunWithContext method.
func (suite *DispatcherTestSuite) TestRunWithContext() {
	// Make some fake SQS messages.
	msg1 := makeMsg("msg1")
	msg2 := makeMsg("msg2")
	msg3 := makeMsg("msg3")
	suite.SUT.MaxConcurrent = 2

	// Create a mock handler that will be call once for each message.
	handler := &mocks.Handler{}
	handler.Test(suite.T())

	// Generate the sequence off message visibility timeouts we expect.
	boff := suite.SUT.backoff()
	timeout1 := boff()
	timeout2 := boff()
	timeout3 := boff()

	totalExpectedRunTime := 0 * time.Nanosecond // Keep track of how long we expect the test to take.

	// Describe sequence of expected mock calls, then call RunWithContext().
	//
	// Step 1: Dispatcher will try to receives messages, gets none within PollTimeout.
	expectedReceiveInput := &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(testQueueURL),
		MaxNumberOfMessages: aws.Int64(int64(suite.SUT.MaxConcurrent)),
		VisibilityTimeout:   aws.Int64(numSeconds(ceilSeconds(suite.SUT.InitialVisibilityTimeout))),
		WaitTimeSeconds:     aws.Int64(numSeconds(suite.SUT.PollTime)),
	}
	suite.MockSQS.On("ReceiveMessageWithContext", AnyContext, expectedReceiveInput).
		After(suite.SUT.PollTime).
		Return(&sqs.ReceiveMessageOutput{Messages: []*sqs.Message{}}, error(nil)).
		Once()
	totalExpectedRunTime += suite.SUT.PollTime

	// Step 2: Dispatcher will try to receives messages again, this time gets the first two.
	suite.MockSQS.On("ReceiveMessageWithContext", AnyContext, expectedReceiveInput).
		After(suite.SUT.PollTime/2).
		Return(&sqs.ReceiveMessageOutput{Messages: []*sqs.Message{msg1, msg2}}, error(nil)).
		Once()
	totalExpectedRunTime += suite.SUT.PollTime / 2

	// Step 3a: handler is called for msg1. It succeeds immediately and gets deleted.
	handler.On("Handle", AnyContext, msg1).
		Return(error(nil)).
		Once()
	suite.MockSQS.On("DeleteMessageWithContext", AnyContext, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(testQueueURL),
		ReceiptHandle: msg1.ReceiptHandle,
	}).Return(&sqs.DeleteMessageOutput{}, error(nil)).Once()

	// Step 3b: handler is called for msg2. It succeeds but only after
	// the visibility timeout needs to be updated, then gets deleted.
	handler.On("Handle", AnyContext, msg2).
		After(timeout1 + timeout2/2). // Return half way through the second timeout
		Return(error(nil)).
		Once()
	suite.MockSQS.On("ChangeMessageVisibilityWithContext", AnyContext, &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          aws.String(testQueueURL),
		ReceiptHandle:     msg2.ReceiptHandle,
		VisibilityTimeout: aws.Int64(numSeconds(ceilSeconds(timeout2))),
	}).Return(&sqs.ChangeMessageVisibilityOutput{}, error(nil)).Once()
	suite.MockSQS.On("DeleteMessageWithContext", AnyContext, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(testQueueURL),
		ReceiptHandle: msg2.ReceiptHandle,
	}).Return(&sqs.DeleteMessageOutput{}, error(nil)).Once()

	// Step 4: Dispatcher will try to receives messages again, this time gets the last one.
	expectedReceiveInput = &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(testQueueURL),
		MaxNumberOfMessages: aws.Int64(1), // 1 because msg2 should still be in progress.
		VisibilityTimeout:   aws.Int64(numSeconds(ceilSeconds(suite.SUT.InitialVisibilityTimeout))),
		WaitTimeSeconds:     aws.Int64(numSeconds(suite.SUT.PollTime)),
	}
	suite.MockSQS.On("ReceiveMessageWithContext", AnyContext, expectedReceiveInput).
		After(suite.SUT.PollTime/2).
		Return(&sqs.ReceiveMessageOutput{Messages: []*sqs.Message{msg3}}, error(nil)).
		Once()
	// Thereafter, calls to ReceiveMessageWithContext return no messages.
	suite.MockSQS.On("ReceiveMessageWithContext", AnyContext, expectedReceiveInput).
		After(suite.SUT.PollTime).
		Return(&sqs.ReceiveMessageOutput{Messages: []*sqs.Message{}}, error(nil)).
		Maybe()
	totalExpectedRunTime += suite.SUT.PollTime / 2

	// Step 5: handler is called for msg3. It fails on the third timeout update.
	msg3Err := errors.New("msg3Err")
	failAfter := timeout1 + timeout2 + timeout3/2 // Return half way through the third timeout.
	handler.On("Handle", AnyContext, msg3).
		After(failAfter).
		Return(msg3Err).
		Once()
	suite.MockSQS.On("ChangeMessageVisibilityWithContext", AnyContext, &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          aws.String(testQueueURL),
		ReceiptHandle:     msg3.ReceiptHandle,
		VisibilityTimeout: aws.Int64(numSeconds(ceilSeconds(timeout2))),
	}).Return(&sqs.ChangeMessageVisibilityOutput{}, error(nil)).Once()
	suite.MockSQS.On("ChangeMessageVisibilityWithContext", AnyContext, &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          aws.String(testQueueURL),
		ReceiptHandle:     msg3.ReceiptHandle,
		VisibilityTimeout: aws.Int64(numSeconds(ceilSeconds(timeout3))),
	}).Return(&sqs.ChangeMessageVisibilityOutput{}, error(nil)).Once()
	totalExpectedRunTime += failAfter

	// Run the Dispatcher with a timeout.
	ctx, cancel := context.WithCancel(context.Background())
	shouldFinishWithin := time.Duration(totalExpectedRunTime * 2) // Add a little buffer time.
	doCancel := time.AfterFunc(shouldFinishWithin, cancel)
	err := suite.SUT.RunWithContext(ctx, handler)

	suite.True(doCancel.Stop(), "timed out")
	suite.Equal(msg3Err, err)
	handler.AssertCalled(suite.T(), "Handle", AnyContext, msg1)
	handler.AssertCalled(suite.T(), "Handle", AnyContext, msg2)
	handler.AssertCalled(suite.T(), "Handle", AnyContext, msg3)
	handler.AssertExpectations(suite.T())
}

// TestReceiveMessages tests the Dispatcher.receiveMessages method.
func (suite *DispatcherTestSuite) TestReceiveMessages() {
	const numMessages = 5
	messages := make([]*sqs.Message, 5)
	for i := range messages {
		messages[i] = makeMsg("msg" + strconv.Itoa(i))
	}

	suite.Run("success", func() {
		suite.MockSQS.On("ReceiveMessageWithContext", AnyContext, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(testQueueURL),
			MaxNumberOfMessages: aws.Int64(numMessages),
			VisibilityTimeout:   aws.Int64(numSeconds(ceilSeconds(suite.SUT.InitialVisibilityTimeout))),
			WaitTimeSeconds:     aws.Int64(numSeconds(suite.SUT.PollTime)),
		}).Return(&sqs.ReceiveMessageOutput{Messages: messages}, error(nil)).Once()
		result, err := suite.SUT.receiveMessages(context.Background(), numMessages)
		suite.NoError(err)
		suite.Equal(messages, result)
	})

	suite.Run("error", func() {
		const errMsg = "a foobar happened"
		suite.MockSQS.On("ReceiveMessageWithContext", AnyContext, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(testQueueURL),
			MaxNumberOfMessages: aws.Int64(numMessages),
			VisibilityTimeout:   aws.Int64(numSeconds(ceilSeconds(suite.SUT.InitialVisibilityTimeout))),
			WaitTimeSeconds:     aws.Int64(numSeconds(suite.SUT.PollTime)),
		}).Return((*sqs.ReceiveMessageOutput)(nil), errors.New(errMsg)).Once()
		result, err := suite.SUT.receiveMessages(context.Background(), numMessages)
		suite.EqualError(err, errMsg)
		suite.Nil(result)
	})
}

// TestDeleteMessage tests the Dispatcher.deleteMessage method.
func (suite *DispatcherTestSuite) TestDeleteMessage() {
	msg := makeMsg("msg")

	suite.Run("success", func() {
		suite.MockSQS.On("DeleteMessageWithContext", AnyContext, &sqs.DeleteMessageInput{
			QueueUrl:      aws.String(testQueueURL),
			ReceiptHandle: msg.ReceiptHandle,
		}).Return(&sqs.DeleteMessageOutput{}, error(nil)).Once()
		err := suite.SUT.deleteMessage(context.Background(), *msg.ReceiptHandle)
		suite.NoError(err)
	})

	suite.Run("error", func() {
		const errMsg = "a foobar happened"
		suite.MockSQS.On("DeleteMessageWithContext", AnyContext, &sqs.DeleteMessageInput{
			QueueUrl:      aws.String(testQueueURL),
			ReceiptHandle: msg.ReceiptHandle,
		}).Return((*sqs.DeleteMessageOutput)(nil), errors.New(errMsg)).Once()
		err := suite.SUT.deleteMessage(context.Background(), *msg.ReceiptHandle)
		suite.EqualError(err, errMsg)
	})
}

// TestUpdateMessageVisibility tests the Dispatcher.updateMessageVisibility method.
func (suite *DispatcherTestSuite) TestUpdateMessageVisibility() {
	msg := makeMsg("msg")
	d := 8 * time.Second

	suite.Run("success", func() {
		suite.MockSQS.On("ChangeMessageVisibilityWithContext", AnyContext, &sqs.ChangeMessageVisibilityInput{
			QueueUrl:          aws.String(testQueueURL),
			ReceiptHandle:     msg.ReceiptHandle,
			VisibilityTimeout: aws.Int64(numSeconds(ceilSeconds(d))),
		}).Return(&sqs.ChangeMessageVisibilityOutput{}, error(nil)).Once()
		err := suite.SUT.updateMessageVisibility(context.Background(), *msg.ReceiptHandle, d)
		suite.NoError(err)
	})

	suite.Run("error", func() {
		const errMsg = "a foobar happened"
		suite.MockSQS.On("ChangeMessageVisibilityWithContext", AnyContext, &sqs.ChangeMessageVisibilityInput{
			QueueUrl:          aws.String(testQueueURL),
			ReceiptHandle:     msg.ReceiptHandle,
			VisibilityTimeout: aws.Int64(numSeconds(ceilSeconds(d))),
		}).Return((*sqs.ChangeMessageVisibilityOutput)(nil), errors.New(errMsg)).Once()
		err := suite.SUT.updateMessageVisibility(context.Background(), *msg.ReceiptHandle, d)
		suite.EqualError(err, errMsg)
	})
}

// TestBackoff tests the Dispatcher.backoff method.
func (suite *DispatcherTestSuite) TestBackoff() {
	boff := suite.SUT.backoff()
	for i := 0; i < 5; i++ {
		suite.Run("loop-"+strconv.Itoa(i), func() {
			want := time.Duration(float64(suite.SUT.InitialVisibilityTimeout) * math.Pow(backoff.DefaultMultiplier, float64(i)))
			want = minD(want, suite.SUT.MaxVisibilityTimeout) // Never more than max tmieout
			got := boff()
			suite.Equal(want, got)
		})
	}
}

// TestNotifyChangeMessageVisibility tests the Dispatcher.notifyChangeMessageVisibility method.
func (suite *DispatcherTestSuite) TestNotifyChangeMessageVisibility() {
	msg := makeMsg("msg")
	c := make(chan changeMessageVisibility)

	// assertReceivedIn asserts that we were able to select from channel c
	// after d +/- delta from now.
	assertReceivedIn := func(d, delta time.Duration) (changeMessageVisibility, bool) {
		now := time.Now()
		begin := d - delta
		end := d + delta
		beginExpected := time.NewTimer(begin)
		select {
		case v, ok := <-c:
			t := time.Now()
			notClosed := suite.True(ok, "Channel c closed.")
			afterStart := suite.False(
				beginExpected.Stop(),
				"Received from channel c before expected (should have been: %s - %s, was: %s).",
				begin.String(),
				end.String(),
				t.Sub(now).String(),
			)
			return v, (notClosed && afterStart)
		case <-time.After(end):
			return changeMessageVisibility{}, suite.True(
				false,
				"Received from channel c after expected (should have been: %s - %s).",
				begin.String(),
				end.String(),
			)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	defer func() {
		cancel()
		wg.Wait() // Wait for notifyChangeMessageVisibility to stop.
	}()

	boff := suite.SUT.backoff()
	wg.Add(1)
	go func() {
		defer wg.Done()
		suite.SUT.notifyChangeMessageVisibility(ctx, *msg.ReceiptHandle, c)
	}()

	noticeArrivesIn := boff() - testAWSCommBuffer
	for i := 0; i < 5; i++ {
		suite.Run("loop-"+strconv.Itoa(i), func() {
			visibilityTimeout := boff()
			if notice, ok := assertReceivedIn(noticeArrivesIn, testSecond/10); ok {
				suite.Equal(*msg.ReceiptHandle, notice.ReceiptHandle)
				suite.InDelta(visibilityTimeout, notice.NewVisibilityTimeout, float64(testAWSCommBuffer))
			}
			noticeArrivesIn = visibilityTimeout - testAWSCommBuffer
		})
	}
}

// TestDispatcher runs the DispatcherTestSuite.
func TestDispatcher(t *testing.T) {
	suite.Run(t, new(DispatcherTestSuite))
}

// AnyContext can be used in mock assertions to test that a context.Context was passed.
var AnyContext = mock.MatchedBy(func(ctx context.Context) bool {
	return true
})

func minD(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
