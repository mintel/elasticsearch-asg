package mocks

import (
	"github.com/stretchr/testify/mock" // Mocking for tests.

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/defaults"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/private/protocol/query"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/sqsiface"
)

type SQS struct {
	mock.Mock
	sqsiface.ClientAPI
}

func (m *SQS) ReceiveMessageRequest(i *sqs.ReceiveMessageInput) sqs.ReceiveMessageRequest {
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
		ServiceName:   sqs.ServiceName,
		ServiceID:     sqs.ServiceID,
		EndpointsID:   sqs.EndpointsID,
		SigningName:   "sqs",
		SigningRegion: cfg.Region,
		APIVersion:    "2012-11-05",
	}

	op := &aws.Operation{
		Name:       "ReceiveMessage",
		HTTPMethod: "POST",
		HTTPPath:   "/",
	}

	req := aws.New(cfg, metadata, cfg.Handlers, nil, op, i, data)
	req.Error = err
	return sqs.ReceiveMessageRequest{
		Input:   i,
		Copy:    m.ReceiveMessageRequest,
		Request: req,
	}
}

func (m *SQS) DeleteMessageBatchRequest(i *sqs.DeleteMessageBatchInput) sqs.DeleteMessageBatchRequest {
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
		ServiceName:   sqs.ServiceName,
		ServiceID:     sqs.ServiceID,
		EndpointsID:   sqs.EndpointsID,
		SigningName:   "sqs",
		SigningRegion: cfg.Region,
		APIVersion:    "2012-11-05",
	}

	op := &aws.Operation{
		Name:       "DeleteMessageBatch",
		HTTPMethod: "POST",
		HTTPPath:   "/",
	}

	req := aws.New(cfg, metadata, cfg.Handlers, nil, op, i, data)
	req.Error = err
	return sqs.DeleteMessageBatchRequest{
		Input:   i,
		Copy:    m.DeleteMessageBatchRequest,
		Request: req,
	}
}
