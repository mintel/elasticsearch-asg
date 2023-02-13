// Code generated by mockery v1.0.0, but then edit down to only the methods needed.

package mocks

import (
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"

	"github.com/stretchr/testify/mock"
)

// EC2API is an autogenerated mock type for the EC2API type
type EC2API struct {
	ec2iface.EC2API
	mock.Mock
}

// DescribeInstances provides a mock function with given fields: _a0
func (_m *EC2API) DescribeInstances(_a0 *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	ret := _m.Called(_a0)

	var r0 *ec2.DescribeInstancesOutput
	if rf, ok := ret.Get(0).(func(*ec2.DescribeInstancesInput) *ec2.DescribeInstancesOutput); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*ec2.DescribeInstancesOutput)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*ec2.DescribeInstancesInput) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}