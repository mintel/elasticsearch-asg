package lifecycle

//go:generate sh -c "mockery -name=AutoScalingAPI -dir=$(go list -f '{{.Dir}}' github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface)"

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"             // Generate UUIDs
	"github.com/stretchr/testify/assert" // Test assertions e.g. equality
	"github.com/stretchr/testify/mock"   // Tools for mocking things

	// AWS clients and stuff.
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"

	"github.com/mintel/elasticsearch-asg/internal/pkg/testutil"
	"github.com/mintel/elasticsearch-asg/pkg/lifecycle/mocks" // Mocked AWS clients.
)

var (
	acctID         = "123456789012"
	instanceID     = "i-123456789"
	reqID          = uuid.New().String()
	token          = uuid.New().String()
	asgName        = "ASGName"
	hookName       = "MyHook"
	transition     = TransitionLaunching
	hTimeout       = 50 * time.Millisecond
	hGlobalTimeout = 100 * hTimeout
)

func setup(t *testing.T) (*mocks.AutoScalingAPI, context.Context, func()) {
	originalCommBufD := commBufD
	originalTimeoutIncrement := timeoutIncrement

	commBufD = 0
	timeoutIncrement = time.Millisecond

	ctx, _, teardown := testutil.ClientTestSetup(t)

	m := &mocks.AutoScalingAPI{}
	m.Test(t)

	teardown2 := func() {
		teardown()
		commBufD = originalCommBufD
		timeoutIncrement = originalTimeoutIncrement
	}
	return m, ctx, teardown2
}

func TestNewEventFromMsg(t *testing.T) {
	m, ctx, teardown := setup(t)
	defer teardown()

	mIn := &autoscaling.DescribeLifecycleHooksInput{
		AutoScalingGroupName: aws.String(asgName),
		LifecycleHookNames:   []*string{aws.String(hookName)},
	}
	mOut := &autoscaling.DescribeLifecycleHooksOutput{
		LifecycleHooks: []*autoscaling.LifecycleHook{
			&autoscaling.LifecycleHook{
				AutoScalingGroupName: aws.String(asgName),
				GlobalTimeout:        aws.Int64(int64(hGlobalTimeout / timeoutIncrement)),
				HeartbeatTimeout:     aws.Int64(int64(hTimeout / timeoutIncrement)),
				LifecycleHookName:    aws.String(hookName),
				LifecycleTransition:  aws.String(transition.String()),
			},
		},
	}
	m.On("DescribeLifecycleHooksWithContext", AnyContext, mIn).Once().Return(mOut, error(nil))

	start := time.Now().UTC().Round(time.Millisecond)
	input := fmt.Sprintf(`{
		"AutoScalingGroupName": "%s",
		"Service": "AWS Auto Scaling",
		"Time": "%s",
		"AccountId": "%s",
		"LifecycleTransition": "%s",
		"RequestId": "%s",
		"LifecycleActionToken": "%s",
		"EC2InstanceId": "%s",
		"LifecycleHookName": "%s"
	}`, asgName, start.Format(time.RFC3339Nano), acctID, transition, reqID, token, instanceID, hookName)
	want := &Event{
		AccountID:              acctID,
		AutoScalingGroupName:   asgName,
		InstanceID:             instanceID,
		LifecycleActionToken:   token,
		GlobalHeartbeatTimeout: hGlobalTimeout,
		HeartbeatTimeout:       hTimeout,
		LifecycleHookName:      hookName,
		LifecycleTransition:    transition,
		Start:                  start,
	}

	result, err := NewEventFromMsg(ctx, m, []byte(input))
	assert.NoError(t, err)
	assert.Equal(t, want, result)
	m.AssertExpectations(t)
}

func TestNewEventFromMsg_testEvent(t *testing.T) {
	m, ctx, teardown := setup(t)
	defer teardown()

	mIn := &autoscaling.DescribeLifecycleHooksInput{
		AutoScalingGroupName: aws.String(asgName),
		LifecycleHookNames:   []*string{aws.String(hookName)},
	}
	mOut := &autoscaling.DescribeLifecycleHooksOutput{
		LifecycleHooks: []*autoscaling.LifecycleHook{
			&autoscaling.LifecycleHook{
				AutoScalingGroupName: aws.String(asgName),
				GlobalTimeout:        aws.Int64(int64(hGlobalTimeout.Seconds())),
				HeartbeatTimeout:     aws.Int64(int64(hTimeout.Seconds())),
				LifecycleHookName:    aws.String(hookName),
				LifecycleTransition:  aws.String(transition.String()),
			},
		},
	}
	m.On("DescribeLifecycleHooksWithContext", AnyContext, mIn).Once().Return(mOut, error(nil))

	start := time.Now().UTC().Round(time.Millisecond)
	input := fmt.Sprintf(`{
		"AutoScalingGroupName": "%s",
		"Service": "AWS Auto Scaling",
		"Time": "%s",
		"AccountId": "%s",
		"LifecycleTransition": "%s",
		"RequestId": "%s",
		"LifecycleActionToken": "%s",
		"EC2InstanceId": "%s",
		"LifecycleHookName": "%s",
		"Event": "autoscaling:TEST_NOTIFICATION"
	}`, asgName, start.Format(time.RFC3339), acctID, transition, reqID, token, instanceID, hookName)

	result, err := NewEventFromMsg(ctx, m, []byte(input))
	assert.Equal(t, ErrTestEvent, err)
	assert.Nil(t, result)
	m.AssertNotCalled(t, "DescribeLifecycleHooksWithContext", AnyContext, mIn)
}

func TestNewEventFromMsg_badTransition(t *testing.T) {
	m, ctx, teardown := setup(t)
	defer teardown()

	mIn := &autoscaling.DescribeLifecycleHooksInput{
		AutoScalingGroupName: aws.String(asgName),
		LifecycleHookNames:   []*string{aws.String(hookName)},
	}
	mOut := &autoscaling.DescribeLifecycleHooksOutput{
		LifecycleHooks: []*autoscaling.LifecycleHook{
			&autoscaling.LifecycleHook{
				AutoScalingGroupName: aws.String(asgName),
				GlobalTimeout:        aws.Int64(int64(hGlobalTimeout.Seconds())),
				HeartbeatTimeout:     aws.Int64(int64(hTimeout.Seconds())),
				LifecycleHookName:    aws.String(hookName),
				LifecycleTransition:  aws.String(transition.String()),
			},
		},
	}
	m.On("DescribeLifecycleHooksWithContext", AnyContext, mIn).Once().Return(mOut, error(nil))

	start := time.Now().UTC().Round(time.Millisecond)
	input := fmt.Sprintf(`{
		"AutoScalingGroupName": "%s",
		"Service": "AWS Auto Scaling",
		"Time": "%s",
		"AccountId": "%s",
		"LifecycleTransition": "autoscaling:EC2_INSTANCE_FROBNOSTICATING",
		"RequestId": "%s",
		"LifecycleActionToken": "%s",
		"EC2InstanceId": "%s",
		"LifecycleHookName": "%s"
	}`, asgName, start.Format(time.RFC3339), acctID, reqID, token, instanceID, hookName)

	result, err := NewEventFromMsg(ctx, m, []byte(input))
	assert.Equal(t, ErrUnknownTransition, err)
	assert.Nil(t, result)
	m.AssertNotCalled(t, "DescribeLifecycleHooksWithContext", AnyContext, mIn)
}

func TestNewEventFromMsg_errUnmarshal(t *testing.T) {
	m, ctx, teardown := setup(t)
	defer teardown()

	mIn := &autoscaling.DescribeLifecycleHooksInput{
		AutoScalingGroupName: aws.String(asgName),
		LifecycleHookNames:   []*string{aws.String(hookName)},
	}
	mOut := &autoscaling.DescribeLifecycleHooksOutput{
		LifecycleHooks: []*autoscaling.LifecycleHook{
			&autoscaling.LifecycleHook{
				AutoScalingGroupName: aws.String(asgName),
				GlobalTimeout:        aws.Int64(int64(hGlobalTimeout.Seconds())),
				HeartbeatTimeout:     aws.Int64(int64(hTimeout.Seconds())),
				LifecycleHookName:    aws.String(hookName),
				LifecycleTransition:  aws.String(transition.String()),
			},
		},
	}
	m.On("DescribeLifecycleHooksWithContext", AnyContext, mIn).Once().Return(mOut, error(nil))

	start := time.Now().UTC().Round(time.Millisecond)
	input := fmt.Sprintf(`{
		"AutoScalingGroupName": "%s",
		"Service": "AWS Auto Scaling",
		"Time": "%s",
		"AccountId": "%s",
		"LifecycleTransition": "%s",
		"RequestId": "%s",
		"LifecycleActionToken": "%s",
		"EC2InstanceId": "%s",
		"LifecycleHookName": "%s",
	}`, asgName, start.Format(time.RFC3339), acctID, transition, reqID, token, instanceID, hookName)

	result, err := NewEventFromMsg(ctx, m, []byte(input))
	assert.Nil(t, result)
	assert.IsType(t, (*json.SyntaxError)(nil), err)
	m.AssertNotCalled(t, "DescribeLifecycleHooksWithContext", AnyContext, mIn)
}

func TestKeepAlive(t *testing.T) {
	m, ctx, teardown := setup(t)
	defer teardown()

	mIn := &autoscaling.RecordLifecycleActionHeartbeatInput{
		AutoScalingGroupName: aws.String(asgName),
		LifecycleHookName:    aws.String(hookName),
		LifecycleActionToken: aws.String(token),
	}
	mOut := &autoscaling.RecordLifecycleActionHeartbeatOutput{}
	m.On("RecordLifecycleActionHeartbeatWithContext", AnyContext, mIn).Once().Return(mOut, error(nil))

	event := &Event{
		AccountID:              acctID,
		AutoScalingGroupName:   asgName,
		InstanceID:             instanceID,
		LifecycleActionToken:   token,
		GlobalHeartbeatTimeout: hGlobalTimeout,
		HeartbeatTimeout:       hTimeout,
		LifecycleHookName:      hookName,
		LifecycleTransition:    transition,
		Start:                  time.Now().UTC().Round(time.Millisecond),
	}

	cond := func(ctx context.Context, e *Event) (bool, error) {
		args := m.MethodCalled("cond", ctx, e)
		if err := ctx.Err(); err != nil {
			return false, err
		}
		return args.Bool(0), args.Error(1)
	}
	m.On("cond", AnyContext, event).Once().Return(false, error(nil)) // Postpone event
	m.On("cond", AnyContext, event).Once().Return(true, error(nil))  // Allow event to continue

	err := KeepAlive(ctx, m, event, cond)

	assert.NoError(t, err)
	m.AssertExpectations(t)
}

// AnyContext can be used in mock assertions to test that a context.Context was passed.
//
//   // func (*SQSAPI) ReceiveMessageWithContext(context.Context, *sqs.ReceiveMessageInput, ...request.Option) (*sqs.ReceiveMessageOutput, error)
//   m.On("ReceiveMessageWithContext", awsmock.AnyContext, input, awsmock.NilOpts)
var AnyContext = mock.MatchedBy(func(ctx context.Context) bool {
	return true
})
