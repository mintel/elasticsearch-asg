package throttler

//go:generate sh -c "mockery -name=ClientAPI -dir=$(go list -f '{{.Dir}}' github.com/aws/aws-sdk-go-v2/service/autoscaling/autoscalingiface)"

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert" // Test assertions e.g. equality.

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"

	"github.com/mintel/elasticsearch-asg/internal/app/throttler/mocks"
	"github.com/mintel/elasticsearch-asg/internal/pkg/testutil" // Testing utilities.
)

func TestNewAutoScalingGroupEnabler(t *testing.T) {
	const (
		group = "MyAutoScalingGroup"
	)

	t.Run("enabled", func(t *testing.T) {
		logger, teardown := testutil.TestLogger(t)
		defer teardown()

		client := &mocks.AutoScaling{}
		client.Test(t)
		client.On("DescribeAutoScalingGroupsRequest", &autoscaling.DescribeAutoScalingGroupsInput{
			AutoScalingGroupNames: []string{group},
		}).Return(
			&autoscaling.DescribeAutoScalingGroupsOutput{
				AutoScalingGroups: []autoscaling.AutoScalingGroup{
					autoscaling.AutoScalingGroup{
						AutoScalingGroupName: aws.String(group),
					},
				},
			},
			error(nil),
		).Once()

		asge, err := NewAutoScalingGroupEnabler(client, logger, false, group)
		if assert.NoError(t, err) {
			assert.True(t, asge.IsEnabled(), "group is disabled when it should be enabled")
		}
		client.AssertExpectations(t)
	})

	t.Run("disabled", func(t *testing.T) {
		logger, teardown := testutil.TestLogger(t)
		defer teardown()

		client := &mocks.AutoScaling{}
		client.Test(t)
		client.On("DescribeAutoScalingGroupsRequest", &autoscaling.DescribeAutoScalingGroupsInput{
			AutoScalingGroupNames: []string{group},
		}).Return(
			&autoscaling.DescribeAutoScalingGroupsOutput{
				AutoScalingGroups: []autoscaling.AutoScalingGroup{
					autoscaling.AutoScalingGroup{
						AutoScalingGroupName: aws.String(group),
						SuspendedProcesses: []autoscaling.SuspendedProcess{
							autoscaling.SuspendedProcess{
								ProcessName:      aws.String("AlarmNotification"),
								SuspensionReason: aws.String("Because I said so."),
							},
						},
					},
				},
			},
			error(nil),
		).Once()

		asge, err := NewAutoScalingGroupEnabler(client, logger, false, group)
		if assert.NoError(t, err) {
			assert.False(t, asge.IsEnabled(), "group is enabled when it should be disabled")
		}
		client.AssertExpectations(t)
	})
}

func TestAutoScalingGroupEnabler_Enable(t *testing.T) {
	logger, teardown := testutil.TestLogger(t)
	defer teardown()

	const (
		group = "MyAutoScalingGroup"
	)
	client := &mocks.AutoScaling{}
	client.Test(t)
	as := newAutoScalingGroupEnabler(client, logger, false, group)
	as.state.SetState("disabled")
	wantErr := errors.New("test error")

	t.Run("enable", func(t *testing.T) {
		client.Test(t)
		assert.False(t, as.IsEnabled())

		client.On("ResumeProcessesRequest", &autoscaling.ResumeProcessesInput{
			AutoScalingGroupName: aws.String(group),
			ScalingProcesses:     []string{"AlarmNotification"},
		}).Return(new(autoscaling.ResumeProcessesOutput), error(nil)).Once()

		err := as.Enable()
		assert.NoError(t, err)
		assert.True(t, as.IsEnabled())
		client.AssertExpectations(t)
	})

	t.Run("idempotent", func(t *testing.T) {
		client.Test(t)
		assert.True(t, as.IsEnabled())

		err := as.Enable()
		assert.NoError(t, err)
		assert.True(t, as.IsEnabled())
		client.AssertExpectations(t)
	})

	t.Run("error", func(t *testing.T) {
		client.Test(t)
		as.state.SetState("disabled")
		assert.False(t, as.IsEnabled())

		client.On("ResumeProcessesRequest", &autoscaling.ResumeProcessesInput{
			AutoScalingGroupName: aws.String(group),
			ScalingProcesses:     []string{"AlarmNotification"},
		}).Return((*autoscaling.ResumeProcessesOutput)(nil), wantErr).Once()

		err := as.Enable()
		assert.Equal(t, wantErr, err)
		assert.False(t, as.IsEnabled())
		client.AssertExpectations(t)
	})
}

func TestAutoScalingGroupEnabler_Disable(t *testing.T) {
	logger, teardown := testutil.TestLogger(t)
	defer teardown()
	const (
		group = "MyAutoScalingGroup"
	)
	client := &mocks.AutoScaling{}
	client.Test(t)
	as := newAutoScalingGroupEnabler(client, logger, false, group)
	as.state.SetState("enabled")
	wantErr := errors.New("test error")

	t.Run("disable", func(t *testing.T) {
		client.Test(t)
		assert.True(t, as.IsEnabled())

		client.On("SuspendProcessesRequest", &autoscaling.SuspendProcessesInput{
			AutoScalingGroupName: aws.String(group),
			ScalingProcesses:     []string{"AlarmNotification"},
		}).Return(new(autoscaling.SuspendProcessesOutput), error(nil)).Once()

		err := as.Disable()
		assert.NoError(t, err)
		assert.False(t, as.IsEnabled())
		client.AssertExpectations(t)
	})

	t.Run("idempotent", func(t *testing.T) {
		client.Test(t)
		assert.False(t, as.IsEnabled())

		err := as.Disable()
		assert.NoError(t, err)
		assert.False(t, as.IsEnabled())
		client.AssertExpectations(t)
	})

	t.Run("error", func(t *testing.T) {
		client.Test(t)
		as.state.SetState("enabled")
		assert.True(t, as.IsEnabled())

		client.On("SuspendProcessesRequest", &autoscaling.SuspendProcessesInput{
			AutoScalingGroupName: aws.String(group),
			ScalingProcesses:     []string{"AlarmNotification"},
		}).Return((*autoscaling.SuspendProcessesOutput)(nil), wantErr).Once()

		err := as.Disable()
		assert.Equal(t, wantErr, err)
		assert.True(t, as.IsEnabled())
		client.AssertExpectations(t)
	})
}
