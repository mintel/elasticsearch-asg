package events

import (
	"reflect"
	"time"
)

func init() {
	MustRegisterDetailType(
		"aws.autoscaling",
		"EC2 Instance Terminate Unsuccessful",
		reflect.TypeOf(AutoScalingLifecycleTerminateUnsuccessful{}),
	)
}

// AutoScalingLifecycleTerminateUnsuccessful is one possible
// value for the CloudWatchEvent.Detail field.
//
//   Amazon EC2 Auto Scaling failed to terminate an instance.
//
// Example:
//
//   {
//       "version": "0",
//       "id": "12345678-1234-1234-1234-123456789012",
//       "detail-type": "EC2 Instance Terminate Unsuccessful",
//       "source": "aws.autoscaling",
//       "account": "123456789012",
//       "time": "yyyy-mm-ddThh:mm:ssZ",
//       "region": "us-west-2",
//       "resources": [
//       "auto-scaling-group-arn",
//           "instance-arn"
//       ],
//       "detail": {
//           "StatusCode": "Failed",
//           "AutoScalingGroupName": "my-auto-scaling-group",
//           "ActivityId": "87654321-4321-4321-4321-210987654321",
//           "Details": {
//               "Availability Zone": "us-west-2b",
//               "Subnet ID": "subnet-12345678"
//           },
//           "RequestId": "12345678-1234-1234-1234-123456789012",
//           "StatusMessage": "message-text",
//           "EndTime": "yyyy-mm-ddThh:mm:ssZ",
//           "EC2InstanceId": "i-1234567890abcdef0",
//           "StartTime": "yyyy-mm-ddThh:mm:ssZ",
//           "Cause": "description-text"
//       }
//   }
//
// See also: https://docs.aws.amazon.com/autoscaling/ec2/userguide/cloud-watch-events.html#terminate-unsuccessful
type AutoScalingLifecycleTerminateUnsuccessful struct {
	// Example: "Failed"
	StatusCode string `json:"StatusCode"`

	// Example: "my-auto-scaling-group"
	AutoScalingGroupName string `json:"AutoScalingGroupName"`

	// Example: "87654321-4321-4321-4321-210987654321"
	ActivityID string `json:"ActivityId"`

	Details AZSubnet `json:"Details"`

	// Example: "12345678-1234-1234-1234-123456789012"
	RequestID string `json:"RequestId"`

	// Example: "message-text"
	StatusMessage string `json:"StatusMessage"`

	// Example: "yyyy-mm-ddThh:mm:ssZ"
	EndTime time.Time `json:"EndTime"`

	// Example: "i-1234567890abcdef0"
	EC2InstanceID string `json:"EC2InstanceId"`

	// Example: "yyyy-mm-ddThh:mm:ssZ"
	StartTime time.Time `json:"StartTime"`

	// Example: "description-text"
	Cause string `json:"Cause"`
}
