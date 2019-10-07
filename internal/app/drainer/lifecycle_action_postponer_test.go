package drainer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert" // Test assertions e.g. equality.

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"

	"github.com/mintel/elasticsearch-asg/internal/app/drainer/mocks"
)

func TestLifecycleActionPostponer_Postpone(t *testing.T) {
	m := &mocks.AutoScaling{}
	m.Test(t)

	p := NewLifecycleActionPostponer(m)

	now := time.Now()
	la := &LifecycleAction{
		AutoScalingGroupName: "my-asg",
		LifecycleHookName:    "my-lifecycle-hook",
		Token:                "87654321-4321-4321-4321-210987654321",
		InstanceID:           "i-1234567890abcdef0",
		LifecycleTransition:  "autoscaling:EC2_INSTANCE_TERMINATING",
		Start:                now,
	}

	hook := autoscaling.LifecycleHook{
		AutoScalingGroupName: aws.String("my-asg"),
		LifecycleHookName:    aws.String("my-lifecycle-hook"),
		DefaultResult:        aws.String("ABANDON"),
		LifecycleTransition:  aws.String("autoscaling:EC2_INSTANCE_TERMINATING"),
		HeartbeatTimeout:     aws.Int64(1),
		GlobalTimeout:        aws.Int64(100),
	}

	m.On("DescribeLifecycleHooksRequest", &autoscaling.DescribeLifecycleHooksInput{
		AutoScalingGroupName: aws.String("my-asg"),
		LifecycleHookNames:   []string{"my-lifecycle-hook"},
	}).
		Return(
			&autoscaling.DescribeLifecycleHooksOutput{
				LifecycleHooks: []autoscaling.LifecycleHook{hook},
			},
			error(nil),
		).
		Once()

	heartbeatInput := &autoscaling.RecordLifecycleActionHeartbeatInput{
		AutoScalingGroupName: aws.String("my-asg"),
		LifecycleHookName:    aws.String("my-lifecycle-hook"),
		InstanceId:           aws.String("i-1234567890abcdef0"),
		LifecycleActionToken: aws.String("87654321-4321-4321-4321-210987654321"),
	}

	m.On("RecordLifecycleActionHeartbeatRequest", heartbeatInput).
		Return(&autoscaling.RecordLifecycleActionHeartbeatOutput{}, error(nil))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errc := make(chan error, 1)
	go func() {
		errc <- p.Postpone(ctx, m, la)
	}()
	time.AfterFunc(2*time.Second, cancel)
	assert.Eventually(t, func() bool {
		select {
		case err := <-errc:
			assert.Equal(t, err, context.Canceled)
			return true
		default:
			return false
		}
	}, 3*time.Second, 10*time.Millisecond)

	// Second call to PostponeLifecycleHookAction should hit the
	// lifecycleHookCache and not need to call DescribeLifecycleHooksRequest.
	errc = make(chan error, 1)
	go func() {
		errc <- p.Postpone(ctx, m, la)
	}()
	assert.Eventually(t, func() bool {
		select {
		case err := <-errc:
			assert.Equal(t, err, context.Canceled)
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)

	m.AssertExpectations(t)
}
