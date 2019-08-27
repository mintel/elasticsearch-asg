package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/defaults"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/private/protocol/ec2query"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/ec2iface"
)

// EC2 mocks ec2iface.ClientAPI.
type EC2 struct {
	mock.Mock
	ec2iface.ClientAPI
}

func (m *EC2) DescribeInstancesRequest(i *ec2.DescribeInstancesInput) ec2.DescribeInstancesRequest {
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
	cfg.Handlers.Build.PushBackNamed(ec2query.BuildHandler)
	cfg.Handlers.Unmarshal.Clear()
	cfg.Handlers.UnmarshalMeta.Clear()
	cfg.Handlers.UnmarshalError.Clear()
	cfg.Handlers.Send.Clear()
	cfg.Handlers.Retry.Clear()
	cfg.Handlers.ValidateResponse.Clear()

	metadata := aws.Metadata{
		ServiceName:   ec2.ServiceName,
		ServiceID:     ec2.ServiceID,
		EndpointsID:   ec2.EndpointsID,
		SigningName:   "ec2",
		SigningRegion: cfg.Region,
		APIVersion:    "2016-11-15",
	}

	op := &aws.Operation{
		Name:       "DescribeInstances",
		HTTPMethod: "POST",
		HTTPPath:   "/",
		Paginator: &aws.Paginator{
			InputTokens:     []string{"NextToken"},
			OutputTokens:    []string{"NextToken"},
			LimitToken:      "MaxResults",
			TruncationToken: "",
		},
	}

	req := aws.New(cfg, metadata, cfg.Handlers, nil, op, i, data)
	req.Error = err
	return ec2.DescribeInstancesRequest{
		Input:   i,
		Copy:    m.DescribeInstancesRequest,
		Request: req,
	}
}
