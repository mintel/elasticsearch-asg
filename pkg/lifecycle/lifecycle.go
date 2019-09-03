// Package lifecycle impelmments unmarshaling of AWS Autoscaling Group Lifecycle Hook
// event messages, and provides a function KeepAlive() that will keep an event in the
// Transition:Wait state until a function returns true.
//
// See: https://docs.aws.amazon.com/autoscaling/ec2/userguide/lifecycle-hooks.html
package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap" // Logging

	// AWS clients and stuff
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"

	"github.com/mintel/elasticsearch-asg/internal/pkg/metrics" // Promtheus metrics
	"github.com/mintel/elasticsearch-asg/pkg/ctxlog"           // Logger from context
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

	// ErrExpired is returned when a lifecycle event should be expried according to its timeout and start timestamp.
	ErrExpired = errors.New("lifecycle event has expired")
)

const subsystem = "lifecycle"

var (
	// Up is a Prometheus metrics tracking number of running KeepAlive's.
	Up = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: metrics.Namespace,
		Subsystem: subsystem,
		Name:      "up",
		Help:      "Number of running KeepAlives.",
	})

	// KeepAliveDuration is a Prometheus metric for the duration of the KeepAlive() function.
	KeepAliveDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: subsystem,
		Name:      "keepalive_duration_seconds",
		Help:      "Duration keeping ASG lifecycle hooks alive.",
		Buckets:   prometheus.DefBuckets, // TODO: Define better buckets
	}, []string{metrics.LabelStatus})

	// KeepAliveCheckDuration is a Prometheus metric for the duration of the KeepAlive() check functions.
	KeepAliveCheckDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: subsystem,
		Name:      "keepalive_check_duration_seconds",
		Help:      "Duration running ASG lifecycle hook keep-alive checks.",
		Buckets:   prometheus.DefBuckets, // TODO: Define better buckets
	}, []string{metrics.LabelStatus})
)

func init() {
	Up.Set(0)
}

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

	// Number of times a heartbeat has been recorded for this event.
	HeartbeatCount int `json:"HeartbeatCount,omitempty"`
}

// NewEventFromMsg creates a new event from a lifecycle message.
func NewEventFromMsg(ctx context.Context, client autoscalingiface.AutoScalingAPI, data []byte) (*Event, error) {
	logger := ctxlog.L(ctx)
	e := new(Event)
	if err := json.Unmarshal(data, e); err != nil {
		return nil, err
	}
	if e.Event == TestEvent {
		return nil, ErrTestEvent
	}
	if e.LifecycleHookName == "" {
		return nil, ErrUnmarshal
	}
	if !(e.LifecycleTransition == TransitionTerminating || e.LifecycleTransition == TransitionLaunching) {
		return nil, ErrUnknownTransition
	}
	logger = logger.With(zap.String("asg_name", e.AutoScalingGroupName), zap.String("name", e.LifecycleHookName))
	logger.Debug("describing lifecycle hook")
	resp, err := client.DescribeLifecycleHooksWithContext(ctx, &autoscaling.DescribeLifecycleHooksInput{
		AutoScalingGroupName: aws.String(e.AutoScalingGroupName),
		LifecycleHookNames:   []*string{aws.String(e.LifecycleHookName)},
	})
	if err != nil {
		logger.Error("error describing lifecycle hook", zap.Error(err))
		return nil, err
	}
	logger.Debug("described lifecycle hook")
	e.HeartbeatTimeout = time.Duration(*resp.LifecycleHooks[0].HeartbeatTimeout) * timeoutIncrement
	e.GlobalHeartbeatTimeout = time.Duration(*resp.LifecycleHooks[0].GlobalTimeout) * timeoutIncrement
	return e, nil
}

// GlobalTimeout returns the time past which the lifecycle transition cannot be delayed.
// The maximum is 172800 seconds (48 hours) or 100 times the heartbeat timeout, whichever is smaller.
func (e *Event) GlobalTimeout() time.Time {
	return e.Start.Add(e.GlobalHeartbeatTimeout)
}

// Timeout returns the time that the lifecycle event will expire.
func (e *Event) Timeout() time.Time {
	t := e.Start.Add(time.Duration(e.HeartbeatCount+1) * e.HeartbeatTimeout)
	gt := e.GlobalTimeout()
	if t.Before(gt) {
		return t
	}
	return gt
}

// KeepAlive keeps a lifecycle event in the Transition:Wait state as long as condition c returns false.
//
// The condition is is only checked just before the lifecycle event is due to expire.
func KeepAlive(ctx context.Context, client autoscalingiface.AutoScalingAPI, e *Event, c func(context.Context, *Event) (bool, error)) error {
	timer := metrics.NewVecTimer(KeepAliveDuration)

	Up.Inc()
	defer Up.Dec()

	ctx = ctxlog.WithName(ctx, "KeepAlive")
	logger := ctxlog.L(ctx)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var stopCheck chan error
	var stopHeartbeat chan error

	timeout := e.Timeout()
	d := time.Until(timeout) - commBufD
	logger.Debug("initial d", zap.Time("timeout", timeout), zap.Duration("d", d))
	if d <= 0 {
		err := ErrExpired
		timer.ObserveErr(err)
		return err
	}
	startCheck := time.After(d)
	for {
		select {
		case <-ctx.Done():
			logger.Debug("context done")
			err := ctx.Err()
			if err == context.Canceled {
				err = nil
			}
			timer.ObserveErr(err)
			return err

		case <-startCheck:
			logger.Debug("case: startCheck")
			startCheck = nil                // Disable startCheck case
			stopCheck = make(chan error, 1) // Enable stopCheck case
			go func() {
				timer := metrics.NewVecTimer(KeepAliveCheckDuration)
				ok, err := c(ctx, e)
				if err != nil {
					logger.Debug("check errored", zap.Error(err))
					timer.ObserveWith(prometheus.Labels{"status": "error"})
					stopCheck <- err // Check errored
				} else if ok {
					logger.Debug("check passed")
					timer.ObserveWith(prometheus.Labels{"status": "success"})
					stopCheck <- nil // Check passed
				} else {
					logger.Debug("check failed")
					timer.ObserveWith(prometheus.Labels{"status": "failed"})
					close(stopCheck) // Check failed
				}
			}()

		case err, ok := <-stopCheck:
			logger.Debug("case: stopCheck", zap.Bool("ok", ok), zap.Error(err))
			if err != nil {
				return err // check errored
			} else if ok {
				return nil // check passed
			} // else check failed

			stopCheck = nil // Disable stop check case

			// Schedule the next check.
			e.HeartbeatCount++
			timeout = e.Timeout()
			d = time.Until(timeout) - commBufD
			logger.Debug("new d", zap.Time("timeout", timeout), zap.Duration("d", d))
			if d <= 0 {
				return ErrExpired
			}
			startCheck = time.After(d) // Enable startCheck case

			// Record the heartbeat
			stopHeartbeat = make(chan error, 1) // Enable stopHeartbeat case
			go func() {
				defer close(stopHeartbeat)
				_, err = client.RecordLifecycleActionHeartbeatWithContext(ctx, &autoscaling.RecordLifecycleActionHeartbeatInput{
					AutoScalingGroupName: aws.String(e.AutoScalingGroupName),
					LifecycleHookName:    aws.String(e.LifecycleHookName),
					LifecycleActionToken: aws.String(e.LifecycleActionToken),
				})
				if err != nil {
					e.HeartbeatCount-- // Undo heartbeat increment
				}
				stopHeartbeat <- err
			}()

		case err := <-stopHeartbeat:
			logger.Debug("case: stopHeartbeat")
			stopHeartbeat = nil // Disable stopHeartbeat case
			if err != nil {
				return err
			}
		}
	}
}
