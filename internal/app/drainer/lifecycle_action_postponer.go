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
)

var (
	// ErrLifecycleActionTimeout is returned by PostponeLifecycleHookAction
	// when the lifecycle action times out (or isn't found in the first place).
	ErrLifecycleActionTimeout = goerrors.New("lifecycle action timed out")

	// ErrTestLifecycleAction is return by NewLifecycleAction when
	// the passed CloudWatchEvent doesn't represent a valid LifecycleAction.
	ErrInvalidLifecycleAction = goerrors.New("invalid lifecycle action")
)

// LifecycleActionPostponer prevents LifecycleActions from timing out.
// See the Postpone method for more details.
type LifecycleActionPostponer struct {
	client             autoscalingiface.ClientAPI
	lifecycleHookCache *ristretto.Cache
}

// NewLifecycleActionPostponer returns a new LifecycleActionPostponer.
func NewLifecycleActionPostponer(client autoscalingiface.ClientAPI) *LifecycleActionPostponer {
	// TODO: move this out of a global variable.
	lifecycleHookCache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 10 * 10,
		MaxCost:     10,
		BufferItems: 8,
		Metrics:     true,
	})
	if err != nil {
		panic(err)
	}
	return &LifecycleActionPostponer{
		client:             client,
		lifecycleHookCache: lifecycleHookCache,
	}
}

// Postpone postpones the timeout of a AWS AutoScaling Group
// Lifecycle Hook action until the context is canceled, an error occurs, or the
// Lifecycle Hook's global timeout is reached.
//
// If the action expires (or can't be found; there's no way to distinguish
// in the AWS API) then ErrLifecycleActionTimeout will be returned.
//
// See also: https://docs.aws.amazon.com/autoscaling/ec2/userguide/lifecycle-hooks.html#lifecycle-hooks-overview
func (lap *LifecycleActionPostponer) Postpone(ctx context.Context, c autoscalingiface.ClientAPI, a *LifecycleAction) error {
	// Get Lifecycle Hook description because we need to know
	// what the timeout for each action it.
	hook, err := lap.describeLifecycleHook(ctx, a.AutoScalingGroupName, a.LifecycleHookName)
	if err != nil {
		return err
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

// describeLifecycleHook fetches a description of an AWS AutoScaling Group
// Lifecycle Hook.
func (lap *LifecycleActionPostponer) describeLifecycleHook(ctx context.Context, groupName, hookName string) (*autoscaling.LifecycleHook, error) {
	var hook *autoscaling.LifecycleHook
	cacheKey := groupName + ":" + hookName
	entry, ok := lap.lifecycleHookCache.Get(cacheKey)
	if ok {
		hook = entry.(*autoscaling.LifecycleHook)
	} else {
		req := lap.client.DescribeLifecycleHooksRequest(&autoscaling.DescribeLifecycleHooksInput{
			AutoScalingGroupName: aws.String(groupName),
			LifecycleHookNames:   []string{hookName},
		})
		resp, err := req.Send(ctx)
		if err != nil {
			return nil, err
		}
		if n := len(resp.LifecycleHooks); n != 1 {
			zap.L().Panic("got wrong number of lifecycle hooks",
				zap.Int("count", n))
		}
		hook = &resp.LifecycleHooks[0]
		lap.lifecycleHookCache.Set(cacheKey, hook, 1)
	}
	return hook, nil
}
