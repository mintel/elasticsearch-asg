// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import context "context"

import mock "github.com/stretchr/testify/mock"

// ElasticsearchCommandService is an autogenerated mock type for the ElasticsearchCommandService type
type ElasticsearchCommandService struct {
	mock.Mock
}

// ClearMasterVotingExclusions provides a mock function with given fields: ctx
func (_m *ElasticsearchCommandService) ClearMasterVotingExclusions(ctx context.Context) error {
	ret := _m.Called(ctx)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Drain provides a mock function with given fields: ctx, nodeName
func (_m *ElasticsearchCommandService) Drain(ctx context.Context, nodeName string) error {
	ret := _m.Called(ctx, nodeName)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, nodeName)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ExcludeMasterVoting provides a mock function with given fields: ctx, nodeName
func (_m *ElasticsearchCommandService) ExcludeMasterVoting(ctx context.Context, nodeName string) error {
	ret := _m.Called(ctx, nodeName)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, nodeName)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Undrain provides a mock function with given fields: ctx, nodeName
func (_m *ElasticsearchCommandService) Undrain(ctx context.Context, nodeName string) error {
	ret := _m.Called(ctx, nodeName)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, nodeName)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}