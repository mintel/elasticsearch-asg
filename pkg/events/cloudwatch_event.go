// Package events is used for unmarshaling AWS CloudWatch Events
// from JSON. At the moment it has only the very limited set of of
// event types needed by elasticsearch-asg, but it could be extended easily
// (and would probably lend itself very well to being generated from
// example JSON files).
//
// For examples of events that come via CloudWatch Events,
// see https://docs.aws.amazon.com/AmazonCloudWatch/latest/events/EventTypes.html
//
// See also: https://docs.aws.amazon.com/AmazonCloudWatch/latest/events/WhatIsCloudWatchEvents.html
package events

import (
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"time"
)

var (
	// ErrInvalidCloudWatchEvent is returned when unmarshaling a
	// CloudWatchEvent and the JSON did not contain an ID or
	// DetailType.
	ErrInvalidCloudWatchEvent = errors.New("invalid CloudWatch event")

	// ErrDetailTypeAlreadyRegistered is returned by RegisterDetailType() when a
	// detail type has already been registered.
	ErrDetailTypeAlreadyRegistered = errors.New("detail type already registered")
)

var _detailRegistry = sync.Map{}

func detailTypeKey(source, detailType string) string {
	return source + ":" + detailType
}

// RegisterDetailType can be used to register custom CloudWatch
// event types. It returns ErrDetailTypeAlreadyRegistered if
// source and detailType has already been registered.
func RegisterDetailType(source, detailType string, t reflect.Type) error {
	key := detailTypeKey(source, detailType)
	if _, loaded := _detailRegistry.LoadOrStore(key, t); loaded {
		return ErrDetailTypeAlreadyRegistered
	}
	return nil
}

// RegisterDetailType can be used to register custom CloudWatch
// event types. It panics if source and detailType has already
// been registered.
func MustRegisterDetailType(source, detailType string, t reflect.Type) {
	if err := RegisterDetailType(source, detailType, t); err != nil {
		panic(err)
	}
}

func newDetail(source, detailType string) interface{} {
	key := detailTypeKey(source, detailType)
	t, ok := _detailRegistry.Load(key)
	if !ok {
		return nil
	}
	d := reflect.New(t.(reflect.Type)).Interface()
	return d
}

// CloudWatchEvent is the outer structure of an event sent via CloudWatch Events.
// It is meant to be unmarshaled via encoding/json.
//
// If the Source and DetailType fields are not defined, unmarshaling will return
// A ErrInvalidCloudWatchEvent.
type CloudWatchEvent struct {
	// Example: "0",
	Version string `json:"version"`

	// Example: "12345678-1234-1234-1234-123456789012",
	ID string `json:"id"`

	// Example: "EC2 Spot Instance Interruption Warning",
	DetailType string `json:"detail-type"`

	// Example: "aws.ec2",
	Source string `json:"source"`

	// Example: "123456789012",
	AccountID string `json:"account"`

	// Example: "2019-09-26T12:55:24Z",
	Time time.Time `json:"time"`

	// Example: "us-east-2",
	Region string `json:"region"`

	// Example: ["arn:aws:ec2:us-east-2:123456789012:instance/i-1234567890abcdef0"],
	Resources []string `json:"resources"`

	// The exact type of Detail depends on the
	// Source and DetailType. If it is a known event type,
	// this will be a pointer to one of the detail type structs
	// in this package. If the event type is not known
	// this will default to a map[string]interface{}.
	Detail interface{} `json:"detail"`
}

// UnmarshalJSON implements the json Unmarshaler interface.
func (e *CloudWatchEvent) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	// Unmarshal first to a RawMessage.
	e.Detail = &json.RawMessage{}
	type JSONEvent CloudWatchEvent // Type alias to use default unmarshaling.
	if err := json.Unmarshal(data, (*JSONEvent)(e)); err != nil {
		return err
	}

	if e.Source == "" || e.DetailType == "" {
		return ErrInvalidCloudWatchEvent
	}

	// Unmarshal Detail into the correct type.
	detailJSON := *e.Detail.(*json.RawMessage)
	e.Detail = newDetail(e.Source, e.DetailType)
	if err := json.Unmarshal(detailJSON, &e.Detail); err != nil {
		return err
	}
	return nil
}
