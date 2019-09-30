package drainer

import (
	"context"
	"testing"
	"time"

	"github.com/dgraph-io/ristretto"     // Cache.
	"github.com/stretchr/testify/assert" // Test assertions e.g. equality.

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"

	"github.com/mintel/elasticsearch-asg/internal/app/drainer/mocks"
	"github.com/mintel/elasticsearch-asg/pkg/events" // AWS CloudWatch Events.
)

func TestNewLifecycleAction(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		input   *events.CloudWatchEvent
		want    *LifecycleAction
		wantErr bool
	}{
		{
			name: "terminate",
			input: &events.CloudWatchEvent{
				Time: now,
				Detail: &events.AutoScalingLifecycleTerminateAction{
					LifecycleActionToken: "87654321-4321-4321-4321-210987654321",
					AutoScalingGroupName: "my-asg",
					LifecycleHookName:    "my-lifecycle-hook",
					EC2InstanceID:        "i-1234567890abcdef0",
					LifecycleTransition:  "autoscaling:EC2_INSTANCE_TERMINATING",
					NotificationMetadata: "additional-info",
				},
			},
			want: &LifecycleAction{
				AutoScalingGroupName: "my-asg",
				LifecycleHookName:    "my-lifecycle-hook",
				Token:                "87654321-4321-4321-4321-210987654321",
				InstanceID:           "i-1234567890abcdef0",
				LifecycleTransition:  "autoscaling:EC2_INSTANCE_TERMINATING",
				Start:                now,
			},
		},
		{
			name: "error",
			input: &events.CloudWatchEvent{
				Time: now,
				Detail: &events.EC2SpotInterruption{
					InstanceID:     "i-1234567890abcdef0",
					InstanceAction: "terminate",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewLifecycleAction(tt.input)
			if tt.wantErr {
				assert.Nil(t, got)
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestPostponeLifecycleHookAction(t *testing.T) {
	m := &mocks.AutoScaling{}
	m.Test(t)

	oldCache := lifecycleHookCache
	defer func() {
		lifecycleHookCache = oldCache
	}()
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 10 * 10,
		MaxCost:     10,
		BufferItems: 8,
		Metrics:     true,
	})
	if err != nil {
		panic(err)
	}
	lifecycleHookCache = cache

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
		errc <- PostponeLifecycleHookAction(ctx, m, la)
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
		errc <- PostponeLifecycleHookAction(ctx, m, la)
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
