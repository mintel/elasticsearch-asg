// Package lifecycle impelmments unmarshalling of AWS Autoscaling Group Lifecycle Hook
// event messages, and provides a KeepAlive function that will keep an event in the
// Transition:Wait state until a function returns true.
//
// See: https://docs.aws.amazon.com/autoscaling/ec2/userguide/lifecycle-hooks.html
package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
)

var (
	// How much time to for checks and communication with AWS will take.
	commBufD = 5 * time.Second

	// Event timeouts are a multiple of this
	timeoutIncrement = time.Second
)

var (
	// ErrTestEvent is returned for new Events if it is a test event.
	ErrTestEvent = errors.New("test event")

	// ErrUnknownTransition is returned for Transitions other than launching or terminating.
	ErrUnknownTransition = errors.New("unknown Transition type")

	// ErrUnmarshal is returned when a unmarshalled JSON string doesn't appear to be a lifecycle event.
	ErrUnmarshal = errors.New("data is not a lifecycle event message")
)

// Transition is an enum representing the possible AWS Autoscaling Group
// transitions that can have a lifecycle hook.
type Transition string

func (lt Transition) String() string {
	return string(lt)
}

const (
	// TransitionLaunching represents an instance that is launching.
	TransitionLaunching Transition = "autoscaling:EC2_INSTANCE_LAUNCHING"

	// TransitionTerminating represents an instance that is terminating.
	TransitionTerminating Transition = "autoscaling:EC2_INSTANCE_TERMINATING"

	// TestEvent is sent by AWS on initial lifecycle hook creation.
	TestEvent = "autoscaling:TEST_NOTIFICATION"
)

// Event represents an AWS Lifecycle Hook event.
type Event struct {
	// The AWS account ID.
	AccountID string `json:"AccountId"`

	// The name of the autoscaling group.
	AutoScalingGroupName string `json:"AutoScalingGroupName"`

	Event string `json:"Event"`

	// The ID of the EC2 instance.
	InstanceID string `json:"EC2InstanceId"`

	// A unique token for this event. Used to record lifecycle heartbeat.
	LifecycleActionToken string `json:"LifecycleActionToken"`

	// The global heartbeat timeout duration.
	// The maximum is 172800 seconds (48 hours) or 100 times the HeartbeatTimeout, whichever is smaller.
	GlobalHeartbeatTimeout time.Duration

	// The initial heartbeat timeout duration.
	HeartbeatTimeout time.Duration

	// The name of the lifecycle hook.
	LifecycleHookName string `json:"LifecycleHookName"`

	// Launching or terminating.
	LifecycleTransition Transition `json:"LifecycleTransition"`

	// The time the event started.
	Start time.Time `json:"Time"`

	heartbeatCount int
}

// NewEventFromMsg creates a new event from a lifecycle message.
func NewEventFromMsg(ctx context.Context, client autoscalingiface.AutoScalingAPI, data []byte) (*Event, error) {
	e := new(Event)
	if err := json.Unmarshal(data, e); err != nil {
		return nil, err
	}
	if e.LifecycleHookName == "" {
		return nil, ErrUnmarshal
	}
	if e.Event == TestEvent {
		return nil, ErrTestEvent
	}
	if !(e.LifecycleTransition == TransitionTerminating || e.LifecycleTransition == TransitionLaunching) {
		return nil, ErrUnknownTransition
	}
	resp, err := client.DescribeLifecycleHooksWithContext(ctx, &autoscaling.DescribeLifecycleHooksInput{
		AutoScalingGroupName: aws.String(e.AutoScalingGroupName),
		LifecycleHookNames:   []*string{aws.String(e.LifecycleHookName)},
	})
	if err != nil {
		return nil, err
	}
	e.HeartbeatTimeout = time.Duration(*resp.LifecycleHooks[0].HeartbeatTimeout) * timeoutIncrement
	e.GlobalHeartbeatTimeout = time.Duration(*resp.LifecycleHooks[0].GlobalTimeout) * timeoutIncrement
	return e, ctx.Err()
}

// GlobalTimeout returns the time past which the lifecycle transition cannot be delayed.
// The maximum is 172800 seconds (48 hours) or 100 times the heartbeat timeout, whichever is smaller.
func (e *Event) GlobalTimeout() time.Time {
	return e.Start.Add(e.GlobalHeartbeatTimeout)
}

// Timeout returns the time that the lifecycle event will expire.
func (e *Event) Timeout() time.Time {
	t := e.Start.Add(time.Duration(e.heartbeatCount+1) * e.HeartbeatTimeout)
	gt := e.GlobalTimeout()
	if t.Before(gt) {
		return t
	}
	return gt
}

// KeepAlive keeps a lifecycle event in the Transition:Wait state as long as condition c returns false.
func KeepAlive(ctx context.Context, client autoscalingiface.AutoScalingAPI, e *Event, c func(context.Context, *Event) (bool, error)) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var stopCheck chan error
	var stopHeartbeat chan error

	d := time.Until(e.Timeout()) - commBufD
	if d < 0 {
		return context.DeadlineExceeded
	}
	startCheck := time.After(d)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-startCheck:
			startCheck = nil                // Disable start check case
			stopCheck = make(chan error, 1) // Enable stop check case
			go func() {
				if ok, err := c(ctx, e); err != nil {
					stopCheck <- err
				} else if ok {
					stopCheck <- nil
				} else {
					close(stopCheck)
				}
			}()

		case err, ok := <-stopCheck:
			stopCheck = nil // Disable stop check case
			if err != nil {
				return err
			} else if ok {
				return nil
			}

			e.heartbeatCount++
			d = time.Until(e.Timeout()) - commBufD
			if d < 0 {
				return context.DeadlineExceeded
			}
			startCheck = time.After(d)

			stopHeartbeat = make(chan error, 1) // Enable stop heartbeat case
			go func() {
				_, err = client.RecordLifecycleActionHeartbeatWithContext(ctx, &autoscaling.RecordLifecycleActionHeartbeatInput{
					AutoScalingGroupName: aws.String(e.AutoScalingGroupName),
					LifecycleHookName:    aws.String(e.LifecycleHookName),
					LifecycleActionToken: aws.String(e.LifecycleActionToken),
				})
				stopHeartbeat <- err
			}()

		case err := <-stopHeartbeat:
			stopHeartbeat = nil // Disable stop heartbeat case
			if err != nil {
				return err
			}
		}
	}
}
