package events

import (
	"reflect"
)

func init() {
	MustRegisterDetailType(
		"aws.autoscaling",
		"EC2 Instance-terminate Lifecycle Action",
		reflect.TypeOf(AutoScalingLifecycleTerminateAction{}),
	)
}

// AutoScalingLifecycleTerminateAction is one possible
// value for the CloudWatchEvent.Detail field.
//
//   Amazon EC2 Auto Scaling moved an instance to a
//   Terminating:Wait state due to a lifecycle hook.
//
// Example:
//
//   {
//       "version": "0",
//       "id": "12345678-1234-1234-1234-123456789012",
//       "detail-type": "EC2 Instance-terminate Lifecycle Action",
//       "source": "aws.autoscaling",
//       "account": "123456789012",
//       "time": "yyyy-mm-ddThh:mm:ssZ",
//       "region": "us-west-2",
//       "resources": [
//           "auto-scaling-group-arn"
//       ],
//       "detail": {
//           "LifecycleActionToken":"87654321-4321-4321-4321-210987654321",
//           "AutoScalingGroupName":"my-asg",
//           "LifecycleHookName":"my-lifecycle-hook",
//           "EC2InstanceId":"i-1234567890abcdef0",
//           "LifecycleTransition":"autoscaling:EC2_INSTANCE_TERMINATING",
//           "NotificationMetadata":"additional-info"
//       }
//   }
//
// See also: https://docs.aws.amazon.com/autoscaling/ec2/userguide/cloud-watch-events.html#terminate-lifecycle-action
type AutoScalingLifecycleTerminateAction struct {
	// Example: "87654321-4321-4321-4321-210987654321"
	LifecycleActionToken string `json:"LifecycleActionToken"`

	// Example: "my-asg"
	AutoScalingGroupName string `json:"AutoScalingGroupName"`

	// Example: "my-lifecycle-hook"
	LifecycleHookName string `json:"LifecycleHookName"`

	// Example: "i-1234567890abcdef0"
	EC2InstanceID string `json:"EC2InstanceId"`

	// Example: "autoscaling:EC2_INSTANCE_TERMINATING"
	LifecycleTransition string `json:"LifecycleTransition"`

	// Example: "additional-info"
	NotificationMetadata string `json:"NotificationMetadata"`
}
