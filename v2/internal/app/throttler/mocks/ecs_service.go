package mocks

import (
	"github.com/stretchr/testify/mock" // Mocking for tests.

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/defaults"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/private/protocol/query"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/ecsiface"
)

// AutoScaling mocks autoscalingiface.ClientAPI.
type ECS struct {
	mock.Mock
	ecsiface.ClientAPI
}

func (m *ECS) DescribeServicesRequest(i *ecs.DescribeServicesInput) ecs.DescribeServicesRequest {
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
		Name:       "DescribeServices",
		HTTPMethod: "POST",
		HTTPPath:   "/",
	}

	req := aws.New(cfg, aws.Metadata{}, cfg.Handlers, nil, op, i, data)

	req.Error = err
	return ecs.DescribeServicesRequest{
		Input:   i,
		Copy:    m.DescribeServicesRequest,
		Request: req,
	}
}
