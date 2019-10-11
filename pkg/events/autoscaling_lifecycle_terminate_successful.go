package events

import (
	"reflect"
	"time"
)

func init() {
	MustRegisterDetailType(
		"aws.autoscaling",
		"EC2 Instance Terminate Successful",
		reflect.TypeOf(AutoScalingLifecycleTerminateSuccessful{}),
	)
}

// AutoScalingLifecycleTerminateSuccessful is one possible
// value for the CloudWatchEvent.Detail field.
//
//   Amazon EC2 Auto Scaling successfully terminated an instance.
//
// Example:
//
//   {
//       "version": "0",
//       "id": "12345678-1234-1234-1234-123456789012",
//       "detail-type": "EC2 Instance Terminate Successful",
//       "source": "aws.autoscaling",
//       "account": "123456789012",
//       "time": "yyyy-mm-ddThh:mm:ssZ",
//       "region": "us-west-2",
//       "resources": [
//       "auto-scaling-group-arn",
//           "instance-arn"
//       ],
//       "detail": {
//           "StatusCode": "InProgress",
//           "Description": "Terminating EC2 instance: i-12345678",
//           "AutoScalingGroupName": "my-auto-scaling-group",
//           "ActivityId": "87654321-4321-4321-4321-210987654321",
//           "Details": {
//               "Availability Zone": "us-west-2b",
//               "Subnet ID": "subnet-12345678"
//           },
//           "RequestId": "12345678-1234-1234-1234-123456789012",
//           "StatusMessage": "",
//           "EndTime": "yyyy-mm-ddThh:mm:ssZ",
//           "EC2InstanceId": "i-1234567890abcdef0",
//           "StartTime": "yyyy-mm-ddThh:mm:ssZ",
//           "Cause": "description-text"
//       }
//   }
//
// See also: https://docs.aws.amazon.com/autoscaling/ec2/userguide/cloud-watch-events.html#terminate-successful
type AutoScalingLifecycleTerminateSuccessful struct {
	// Example: "InProgress"
	StatusCode string `json:"StatusCode"`

	// Example: "Terminating EC2 instance: i-12345678"
	Description string `json:"Description"`

	// Example: "my-auto-scaling-group"
	AutoScalingGroupName string `json:"AutoScalingGroupName"`

	// Example: "87654321-4321-4321-4321-210987654321"
	ActivityID string `json:"ActivityId"`

	Details AZSubnet `json:"Details"`

	// Example: "12345678-1234-1234-1234-123456789012"
	RequestID string `json:"RequestId"`

	// Example: ""
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
