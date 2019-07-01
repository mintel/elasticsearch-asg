//go:generate sh -c "mockery -output=./ -outpkg=mocks -name=EC2API         -dir=$(go list -f '{{.Dir}}' github.com/aws/aws-sdk-go/service/ec2/ec2iface)"
//go:generate sh -c "mockery -output=./ -outpkg=mocks -name=SQSAPI         -dir=$(go list -f '{{.Dir}}' github.com/aws/aws-sdk-go/service/sqs/sqsiface)"
//go:generate sh -c "mockery -output=./ -outpkg=mocks -name=AutoScalingAPI -dir=$(go list -f '{{.Dir}}' github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface)"

// Package mocks various mockable things using https://godoc.org/github.com/stretchr/testify/mock.
package mocks
