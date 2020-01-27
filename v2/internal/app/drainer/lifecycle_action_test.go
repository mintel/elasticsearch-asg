package drainer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert" // Test assertions e.g. equality.

	"github.com/mintel/elasticsearch-asg/v2/pkg/events" // AWS CloudWatch Events.
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
