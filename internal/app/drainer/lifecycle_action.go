package drainer

import (
	"time"

	"github.com/mintel/elasticsearch-asg/pkg/events" // AWS CloudWatch Events.
)

// LifecycleAction contains information about on on-going
// AWS AutoScaling Group scaling event, as related to a
// Lifecycle Hook on that Group.
//
// See also: https://docs.aws.amazon.com/autoscaling/ec2/userguide/lifecycle-hooks.html#lifecycle-hooks-overview
type LifecycleAction struct {
	// The name of the AWS AutoScaling Group.
	AutoScalingGroupName string

	// The name of the Lifecycle Hook attached
	// to the AutoScaling Group.
	LifecycleHookName string

	// A unique token (UUID) identifing this particular
	// autoscaling action.
	Token string

	// The ID of the EC2 instance effected by the
	// autoscaling action.
	InstanceID string

	// One of: "autoscaling:EC2_INSTANCE_LAUNCHING", "autoscaling:EC2_INSTANCE_TERMINATING".
	LifecycleTransition string

	// The time the autoscaling action started.
	Start time.Time
}

// NewLifecycleAction returns a new LifecycleAction from a CloudWatchEvent.
// It will return ErrInvalidLifecycleAction if the event doesn't represent
// a valid LifecycleAction.
func NewLifecycleAction(e *events.CloudWatchEvent) (*LifecycleAction, error) {
	la := &LifecycleAction{
		Start: e.Time,
	}
	switch d := e.Detail.(type) {
	// TODO: Add Launching action.
	case *events.AutoScalingLifecycleTerminateAction:
		la.AutoScalingGroupName = d.AutoScalingGroupName
		la.LifecycleHookName = d.LifecycleHookName
		la.Token = d.LifecycleActionToken
		la.InstanceID = d.EC2InstanceID
		la.LifecycleTransition = d.LifecycleTransition
	default:
		return nil, ErrInvalidLifecycleAction
	}
	return la, nil
}
