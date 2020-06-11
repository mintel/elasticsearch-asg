package mocks

import (
	"github.com/stretchr/testify/mock" // Mocking for tests.

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/defaults"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/private/protocol/query"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/autoscalingiface"
)

// AutoScaling mocks autoscalingiface.ClientAPI.
type AutoScaling struct {
	mock.Mock
	autoscalingiface.ClientAPI
}

func (m *AutoScaling) DescribeAutoScalingGroupsRequest(i *autoscaling.DescribeAutoScalingGroupsInput) autoscaling.DescribeAutoScalingGroupsRequest {
	ret := m.Called(i)
	data := ret.Get(0)
	err := ret.Error(1)

	cfg := defaults.Config()
	cfg.Region = "mock-region"
	cfg.EndpointResolver = aws.ResolveWithEndpointURL("https://endpoint")
	cfg.Credentials = aws.StaticCredentialsProvider{
		Value: aws.Credentials{
			AccessKeyID: "AKID", SecretAccessKey: "SECRET", SessionToken: "SESSION",
			Source: "unit test credentials",
		},
	}
	cfg.Handlers.Sign.PushBackNamed(v4.SignRequestHandler)
	cfg.Handlers.Build.PushBackNamed(query.BuildHandler)
	cfg.Handlers.Unmarshal.Clear()
	cfg.Handlers.UnmarshalMeta.Clear()
	cfg.Handlers.UnmarshalError.Clear()
	cfg.Handlers.Send.Clear()
	cfg.Handlers.Retry.Clear()
	cfg.Handlers.ValidateResponse.Clear()

	op := &aws.Operation{
		Name:       "DescribeAutoScalingGroups",
		HTTPMethod: "POST",
		HTTPPath:   "/",
		Paginator: &aws.Paginator{
			InputTokens:     []string{"NextToken"},
			OutputTokens:    []string{"NextToken"},
			LimitToken:      "MaxRecords",
			TruncationToken: "",
		},
	}

	req := aws.New(cfg, aws.Metadata{}, cfg.Handlers, nil, op, i, data)
	req.Error = err
	return autoscaling.DescribeAutoScalingGroupsRequest{
		Input:   i,
		Copy:    m.DescribeAutoScalingGroupsRequest,
		Request: req,
	}
}

func (m *AutoScaling) ResumeProcessesRequest(i *autoscaling.ResumeProcessesInput) autoscaling.ResumeProcessesRequest {
	ret := m.Called(i)
	data := ret.Get(0)
	err := ret.Error(1)

	cfg := defaults.Config()
	cfg.Region = "mock-region"
	cfg.EndpointResolver = aws.ResolveWithEndpointURL("https://endpoint")
	cfg.Credentials = aws.StaticCredentialsProvider{
		Value: aws.Credentials{
			AccessKeyID: "AKID", SecretAccessKey: "SECRET", SessionToken: "SESSION",
			Source: "unit test credentials",
		},
	}
	cfg.Handlers.Sign.PushBackNamed(v4.SignRequestHandler)
	cfg.Handlers.Build.PushBackNamed(query.BuildHandler)
	cfg.Handlers.Unmarshal.Clear()
	cfg.Handlers.UnmarshalMeta.Clear()
	cfg.Handlers.UnmarshalError.Clear()
	cfg.Handlers.Send.Clear()
	cfg.Handlers.Retry.Clear()
	cfg.Handlers.ValidateResponse.Clear()

	metadata := aws.Metadata{
		ServiceName:   autoscaling.ServiceName,
		ServiceID:     autoscaling.ServiceID,
		EndpointsID:   autoscaling.EndpointsID,
		SigningName:   "autoscaling",
		SigningRegion: cfg.Region,
		APIVersion:    "2011-01-01",
	}

	op := &aws.Operation{
		Name:       "ResumeProcesses",
		HTTPMethod: "POST",
		HTTPPath:   "/",
	}

	req := aws.New(cfg, metadata, cfg.Handlers, nil, op, i, data)
	req.Error = err
	return autoscaling.ResumeProcessesRequest{
		Input:   i,
		Copy:    m.ResumeProcessesRequest,
		Request: req,
	}
}

func (m *AutoScaling) SuspendProcessesRequest(i *autoscaling.SuspendProcessesInput) autoscaling.SuspendProcessesRequest {
	ret := m.Called(i)
	data := ret.Get(0)
	err := ret.Error(1)

	cfg := defaults.Config()
	cfg.Region = "mock-region"
	cfg.EndpointResolver = aws.ResolveWithEndpointURL("https://endpoint")
	cfg.Credentials = aws.StaticCredentialsProvider{
		Value: aws.Credentials{
			AccessKeyID: "AKID", SecretAccessKey: "SECRET", SessionToken: "SESSION",
			Source: "unit test credentials",
		},
	}
	cfg.Handlers.Sign.PushBackNamed(v4.SignRequestHandler)
	cfg.Handlers.Build.PushBackNamed(query.BuildHandler)
	cfg.Handlers.Unmarshal.Clear()
	cfg.Handlers.UnmarshalMeta.Clear()
	cfg.Handlers.UnmarshalError.Clear()
	cfg.Handlers.Send.Clear()
	cfg.Handlers.Retry.Clear()
	cfg.Handlers.ValidateResponse.Clear()

	metadata := aws.Metadata{
		ServiceName:   autoscaling.ServiceName,
		ServiceID:     autoscaling.ServiceID,
		EndpointsID:   autoscaling.EndpointsID,
		SigningName:   "autoscaling",
		SigningRegion: cfg.Region,
		APIVersion:    "2011-01-01",
	}

	op := &aws.Operation{
		Name:       "SuspendProcesses",
		HTTPMethod: "POST",
		HTTPPath:   "/",
	}

	req := aws.New(cfg, metadata, cfg.Handlers, nil, op, i, data)
	req.Error = err
	return autoscaling.SuspendProcessesRequest{
		Input:   i,
		Copy:    m.SuspendProcessesRequest,
		Request: req,
	}
}
