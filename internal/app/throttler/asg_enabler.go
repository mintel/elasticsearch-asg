package throttler

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/looplab/fsm"
)

type asgEnablerClient interface {
	DescribeAutoScalingGroupsRequest(*autoscaling.DescribeAutoScalingGroupsInput) autoscaling.DescribeAutoScalingGroupsRequest
	ResumeProcessesRequest(*autoscaling.ResumeProcessesInput) autoscaling.ResumeProcessesRequest
	SuspendProcessesRequest(*autoscaling.SuspendProcessesInput) autoscaling.SuspendProcessesRequest
}

// AutoScalingGroupEnabler enables or disabled the scaling actions
// of an AWS AutoScaling Group. It does this by enabling/disabling the
// "AlarmNotification" process so the group doesn't react to scaling alarms.
//
// See also: https://docs.aws.amazon.com/autoscaling/ec2/userguide/as-suspend-resume-processes.html
type AutoScalingGroupEnabler struct {
	client asgEnablerClient
	group  string
	state  *fsm.FSM
	dryRun bool
	log    *zap.Logger
}

// NewAutoScalingGroupEnabler returns a new AutoScalingGroupEnabler.
func NewAutoScalingGroupEnabler(client asgEnablerClient, log *zap.Logger, dryRun bool, group string) (*AutoScalingGroupEnabler, error) {
	asge := newAutoScalingGroupEnabler(client, log, dryRun, group)

	// Get current state from AWS.
	req := client.DescribeAutoScalingGroupsRequest(&autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []string{group},
	})
	resp, err := req.Send(context.Background())
	if err != nil {
		return nil, err
	}
	if len(resp.AutoScalingGroups) != 1 {
		panic(fmt.Sprintf("got %d groups", len(resp.AutoScalingGroups)))
	}
	g := resp.AutoScalingGroups[0]
	if n := *g.AutoScalingGroupName; n != group {
		panic(fmt.Sprintf("got group named '%s'", n))
	}
	for _, p := range g.SuspendedProcesses {
		if *p.ProcessName == "AlarmNotification" {
			asge.state.SetState("disabled")
			break
		}
	}

	return asge, nil
}

// newAutoScalingGroupEnabler returns a new AutoScalingGroupEnabler.
// It's separated from NewAutoScalingGroupEnabler for testing purposes.
func newAutoScalingGroupEnabler(client asgEnablerClient, log *zap.Logger, dryRun bool, group string) *AutoScalingGroupEnabler {
	asge := &AutoScalingGroupEnabler{
		client: client,
		group:  group,
		log:    log,
		dryRun: dryRun,
	}
	asge.state = fsm.NewFSM(
		"enabled",
		[]fsm.EventDesc{
			{Name: "enable", Src: []string{"disabled"}, Dst: "enabled"},
			{Name: "disable", Src: []string{"enabled"}, Dst: "disabled"},
		},
		map[string]fsm.Callback{
			"before_enable":  asge.beforeEnable,
			"before_disable": asge.beforeDisable,
		},
	)
	return asge
}

// IsEnabled returns true if scaling is enabled.
func (asge *AutoScalingGroupEnabler) IsEnabled() bool {
	return asge.state.Is("enabled")
}

// Enable enables scaling actions for the AutoScaling Group.
// Subsequent calls to Enable are idempotent.
func (asge *AutoScalingGroupEnabler) Enable() error {
	err := asge.state.Event("enable")
	err = rationalizeFSMError(err)
	return err
}

// Enable disables scaling actions for the AutoScaling Group.
// Subsequent calls to Disable are idempotent.
func (asge *AutoScalingGroupEnabler) Disable() error {
	err := asge.state.Event("disable")
	err = rationalizeFSMError(err)
	return err
}

func (asge *AutoScalingGroupEnabler) beforeEnable(e *fsm.Event) {
	asge.log.Info("enabling autoscaling",
		zap.String("autoscaling_group", asge.group))

	if asge.dryRun {
		return
	}

	req := asge.client.ResumeProcessesRequest(&autoscaling.ResumeProcessesInput{
		AutoScalingGroupName: &asge.group,
		ScalingProcesses:     []string{"AlarmNotification"},
	})
	if _, err := req.Send(context.Background()); err != nil {
		e.Cancel(err)
	}
}

func (asge *AutoScalingGroupEnabler) beforeDisable(e *fsm.Event) {
	asge.log.Info("disabling autoscaling",
		zap.String("autoscaling_group", asge.group))

	if asge.dryRun {
		return
	}

	req := asge.client.SuspendProcessesRequest(&autoscaling.SuspendProcessesInput{
		AutoScalingGroupName: &asge.group,
		ScalingProcesses:     []string{"AlarmNotification"},
	})
	if _, err := req.Send(context.Background()); err != nil {
		e.Cancel(err)
	}
}

func rationalizeFSMError(err error) error {
	switch e := err.(type) {
	// Ignore transitions errors for idempotency.
	case fsm.NoTransitionError, *fsm.NoTransitionError, fsm.InvalidEventError, *fsm.InvalidEventError:
		err = nil

	// Unwrap CanceledError.
	case fsm.CanceledError:
		err = rationalizeFSMError(e.Err)
	case *fsm.CanceledError:
		err = rationalizeFSMError(e.Err)
	}
	return err
}
