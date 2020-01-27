package mocks

import (
	"github.com/stretchr/testify/mock" // Mocking for tests.

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/defaults"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/private/protocol/query"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/cloudwatchiface"
)

// CloudWatch mocks cloudwatchiface.ClientAPI.
type CloudWatch struct {
	mock.Mock
	cloudwatchiface.ClientAPI
}

func (m *CloudWatch) PutMetricDataRequest(i *cloudwatch.PutMetricDataInput) cloudwatch.PutMetricDataRequest {
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
		ServiceName:   cloudwatch.ServiceName,
		ServiceID:     cloudwatch.ServiceID,
		EndpointsID:   cloudwatch.EndpointsID,
		SigningName:   "monitoring",
		SigningRegion: cfg.Region,
		APIVersion:    "2010-08-01",
	}

	op := &aws.Operation{
		Name:       "PutMetricData",
		HTTPMethod: "POST",
		HTTPPath:   "/",
	}

	req := aws.New(cfg, metadata, cfg.Handlers, nil, op, i, data)
	req.Error = err
	return cloudwatch.PutMetricDataRequest{
		Input:   i,
		Copy:    m.PutMetricDataRequest,
		Request: req,
	}
}
