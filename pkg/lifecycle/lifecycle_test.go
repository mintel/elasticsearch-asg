package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"

	"github.com/mintel/elasticsearch-asg/mocks"
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

func setup(t *testing.T) (*mocks.AutoScalingAPI, context.Context, func(), func()) {
	logger := zaptest.NewLogger(t)
	f1 := zap.ReplaceGlobals(logger)
	f2 := zap.RedirectStdLog(logger)

	ctx, cancel := context.WithCancel(context.Background())

	originalCommBufD := commBufD
	originalTimeoutIncrement := timeoutIncrement

	commBufD = 0
	timeoutIncrement = time.Millisecond

	m := &mocks.AutoScalingAPI{}
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
	m.On("DescribeLifecycleHooksWithContext", mocks.AnyContext, mIn).Once().Return(mOut, error(nil))

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
	m.On("DescribeLifecycleHooksWithContext", mocks.AnyContext, mIn).Once().Return(mOut, error(nil))

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
	m.AssertNotCalled(t, "DescribeLifecycleHooksWithContext", mocks.AnyContext, mIn)
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
	m.On("DescribeLifecycleHooksWithContext", mocks.AnyContext, mIn).Once().Return(mOut, error(nil))

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
	m.AssertNotCalled(t, "DescribeLifecycleHooksWithContext", mocks.AnyContext, mIn)
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
	m.On("DescribeLifecycleHooksWithContext", mocks.AnyContext, mIn).Once().Return(mOut, error(nil))

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
	m.AssertNotCalled(t, "DescribeLifecycleHooksWithContext", mocks.AnyContext, mIn)
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
	m.On("RecordLifecycleActionHeartbeatWithContext", mocks.AnyContext, mIn).Once().Return(mOut, error(nil))

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
	m.On("cond", mocks.AnyContext, event).Once().Return(false, error(nil)) // Postpone event
	m.On("cond", mocks.AnyContext, event).Once().Return(true, error(nil))  // Allow event to continue

	err := KeepAlive(ctx, m, event, cond)

	assert.NoError(t, err)
	m.AssertExpectations(t)
}
