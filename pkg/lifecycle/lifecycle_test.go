package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
)

var (
	acctID         = "123456789012"
	instanceID     = "i-123456789"
	reqID          = uuid.New().String()
	token          = uuid.New().String()
	asgName        = "ASGName"
	hookName       = "MyHook"
	transition     = TransitionLaunching
	hTimeout       = 10 * time.Millisecond
	hGlobalTimeout = 100 * hTimeout
)

type mockAutoscalingClient struct {
	mock.Mock
	autoscalingiface.AutoScalingAPI
}

func (m *mockAutoscalingClient) DescribeLifecycleHooksWithContext(ctx aws.Context, input *autoscaling.DescribeLifecycleHooksInput, opts ...request.Option) (*autoscaling.DescribeLifecycleHooksOutput, error) {
	args := m.Called(ctx, input, opts)
	return args.Get(0).(*autoscaling.DescribeLifecycleHooksOutput), args.Error(1)
}

func (m *mockAutoscalingClient) RecordLifecycleActionHeartbeatWithContext(ctx aws.Context, input *autoscaling.RecordLifecycleActionHeartbeatInput, opts ...request.Option) (*autoscaling.RecordLifecycleActionHeartbeatOutput, error) {
	args := m.Called(ctx, input, opts)
	return args.Get(0).(*autoscaling.RecordLifecycleActionHeartbeatOutput), args.Error(1)
}

func setup(t *testing.T) (*mockAutoscalingClient, context.Context, func(), func()) {
	logger := zaptest.NewLogger(t)
	f1 := zap.ReplaceGlobals(logger)
	f2 := zap.RedirectStdLog(logger)

	ctx, cancel := context.WithCancel(context.Background())

	originalCommBufD := commBufD
	originalTimeoutIncrement := timeoutIncrement

	commBufD = 0
	timeoutIncrement = time.Millisecond

	m := &mockAutoscalingClient{}
	m.Test(t)

	teardown := func() {
		cancel()
		f2()
		f1()
		commBufD = originalCommBufD
		timeoutIncrement = originalTimeoutIncrement
		if err := logger.Sync(); err != nil {
			panic(err)
		}
	}
	return m, ctx, cancel, teardown
}

func TestNewEventFromMsg(t *testing.T) {
	m, ctx, _, teardown := setup(t)
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
	m.On("DescribeLifecycleHooksWithContext", anyCtx, mIn, nilReqOpts).Once().Return(mOut, error(nil))

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
	m.AssertCalled(t, "DescribeLifecycleHooksWithContext", anyCtx, mIn, nilReqOpts)
}

func TestNewEventFromMsg_testEvent(t *testing.T) {
	m, ctx, _, teardown := setup(t)
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
	m.On("DescribeLifecycleHooksWithContext", anyCtx, mIn, nilReqOpts).Once().Return(mOut, error(nil))

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
	m.AssertNotCalled(t, "DescribeLifecycleHooksWithContext", anyCtx, mIn, nilReqOpts)
}

func TestNewEventFromMsg_badTransition(t *testing.T) {
	m, ctx, _, teardown := setup(t)
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
	m.On("DescribeLifecycleHooksWithContext", anyCtx, mIn, nilReqOpts).Once().Return(mOut, error(nil))

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
	m.AssertNotCalled(t, "DescribeLifecycleHooksWithContext", anyCtx, mIn, nilReqOpts)
}

func TestNewEventFromMsg_errUnmarshal(t *testing.T) {
	m, ctx, _, teardown := setup(t)
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
	m.On("DescribeLifecycleHooksWithContext", anyCtx, mIn, nilReqOpts).Once().Return(mOut, error(nil))

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
	m.AssertNotCalled(t, "DescribeLifecycleHooksWithContext", anyCtx, mIn, nilReqOpts)
}

func TestKeepAlive(t *testing.T) {
	m, ctx, _, teardown := setup(t)
	defer teardown()

	mIn := &autoscaling.RecordLifecycleActionHeartbeatInput{
		AutoScalingGroupName: aws.String(asgName),
		LifecycleHookName:    aws.String(hookName),
		LifecycleActionToken: aws.String(token),
	}
	mOut := &autoscaling.RecordLifecycleActionHeartbeatOutput{}
	m.On("RecordLifecycleActionHeartbeatWithContext", anyCtx, mIn, nilReqOpts).Once().Return(mOut, error(nil))

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
	m.On("cond", anyCtx, event).Once().Return(false, error(nil)) // Postpone event
	m.On("cond", anyCtx, event).Once().Return(true, error(nil))  // Allow event to continue

	err := KeepAlive(ctx, m, event, cond)

	assert.NoError(t, err)
	m.AssertCalled(t, "RecordLifecycleActionHeartbeatWithContext", anyCtx, mIn, nilReqOpts)
	m.AssertCalled(t, "cond", anyCtx, event)
}

var nilReqOpts []request.Option

var anyCtx = mock.MatchedBy(func(ctx context.Context) bool {
	return true
})
