package drainer

import (
	"context"
	goerrors "errors"
	"strings"
	"time"

	"github.com/dgraph-io/ristretto" // Cache.
	"go.uber.org/zap"                // Logging.

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/autoscalingiface"

	"github.com/mintel/elasticsearch-asg/pkg/events" // AWS CloudWatch Events.
)

var (
	lifecycleHookCache *ristretto.Cache

	// ErrLifecycleActionTimeout is returned by PostponeLifecycleHookAction
	// when the lifecycle action times out (or isn't found in the first place).
	ErrLifecycleActionTimeout = goerrors.New("lifecycle action timed out")

	// ErrTestLifecycleAction is return by NewLifecycleAction when
	// the passed CloudWatchEvent doesn't represent a valid LifecycleAction.
	ErrInvalidLifecycleAction = goerrors.New("invalid lifecycle action")
)

func init() {
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
}

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

// PostponeLifecycleHookAction postpones the timeout of a AWS AutoScaling Group
// Lifecycle Hook action until the context is canceled, an error occurs, or the
// Lifecycle Hook's global timeout is reached.
//
// If the action expires (or can't be found; there's no way to distinguish
// in the AWS API) then ErrLifecycleActionTimeout will be returned.
func PostponeLifecycleHookAction(ctx context.Context, c autoscalingiface.ClientAPI, a *LifecycleAction) error {
	// Get Lifecycle Hook description because we need to know
	// what the timeout for each action it.
	var hook *autoscaling.LifecycleHook
	cacheKey := a.AutoScalingGroupName + ":" + a.LifecycleHookName
	entry, ok := lifecycleHookCache.Get(cacheKey)
	if ok {
		hook = entry.(*autoscaling.LifecycleHook)
	} else {
		req := c.DescribeLifecycleHooksRequest(&autoscaling.DescribeLifecycleHooksInput{
			AutoScalingGroupName: aws.String(a.AutoScalingGroupName),
			LifecycleHookNames:   []string{a.LifecycleHookName},
		})
		resp, err := req.Send(ctx)
		if err != nil {
			return err
		}
		if n := len(resp.LifecycleHooks); n != 1 {
			zap.L().Panic("got wrong number of lifecycle hooks",
				zap.Int("count", n))
		}
		hook = &resp.LifecycleHooks[0]
		lifecycleHookCache.Set(cacheKey, hook, 1)
	}

	timeoutD := time.Duration(aws.Int64Value(hook.HeartbeatTimeout)) * time.Second
	globalTimeoutD := time.Duration(aws.Int64Value(hook.GlobalTimeout)) * time.Second
	timeout := a.Start.Add(timeoutD)
	globalTimeout := time.NewTimer(globalTimeoutD)
	defer globalTimeout.Stop()
	halfWayToTimeout := time.NewTimer(timeout.Sub(time.Now()) / 2)
	defer halfWayToTimeout.Stop()

	heartbeatInput := &autoscaling.RecordLifecycleActionHeartbeatInput{
		AutoScalingGroupName: aws.String(a.AutoScalingGroupName),
		LifecycleHookName:    aws.String(a.LifecycleHookName),
	}
	if a.InstanceID != "" {
		heartbeatInput.InstanceId = aws.String(a.InstanceID)
	}
	if a.Token != "" {
		heartbeatInput.LifecycleActionToken = aws.String(a.Token)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-halfWayToTimeout.C:
			req := c.RecordLifecycleActionHeartbeatRequest(heartbeatInput)
			_, err := req.Send(ctx)
			if aerr, ok := err.(awserr.Error); ok {
				code := aerr.Code()
				msg := aerr.Message()
				if code == "ValidationError" && strings.HasPrefix(msg, "No active Lifecycle Action found with token") {
					return ErrLifecycleActionTimeout
				}
				return err
			} else if err != nil {
				return err
			}
			timeout = timeout.Add(timeoutD)
			halfWayToTimeout.Reset(timeout.Sub(time.Now()) / 2)

		case <-globalTimeout.C:
			return ErrLifecycleActionTimeout
		}
	}
}
