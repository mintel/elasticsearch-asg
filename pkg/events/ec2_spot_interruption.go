package events

import (
	"reflect"
)

func init() {
	MustRegisterDetailType(
		"aws.ec2",
		"EC2 Spot Instance Interruption Warning",
		reflect.TypeOf(EC2SpotInterruption{}),
	)
}

// SpotInterruptionEventDetail is one possible value for the
// CloudWatchEvent.Detail field.
//
//   When Amazon EC2 is going to interrupt your Spot Instance,
//   it emits an event two minutes prior to the actual interruption.
//
// Example:
//
//   {
//       "version": "0",
//       "id": "12345678-1234-1234-1234-123456789012",
//       "detail-type": "EC2 Spot Instance Interruption Warning",
//       "source": "aws.ec2",
//       "account": "123456789012",
//       "time": "yyyy-mm-ddThh:mm:ssZ",
//       "region": "us-east-2",
//       "resources": ["arn:aws:ec2:us-east-2:123456789012:instance/i-1234567890abcdef0"],
//       "detail": {
//           "instance-id": "i-1234567890abcdef0",
//           "instance-action": "action"
//       }
//   }
//
// See also: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-interruptions.html#spot-instance-termination-notices
type EC2SpotInterruption struct {
	// The ID of the EC2 spot instance that is about
	// to be interrupted.
	InstanceID string `json:"instance-id"`

	// One of: "hibernate", "stop", "terminate".
	InstanceAction string `json:"instance-action"`
}
