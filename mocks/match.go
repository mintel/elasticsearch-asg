package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// AnyContext can be used in mock assertions to test that a context.Context was passed.
//
//   // func (*SQSAPI) ReceiveMessageWithContext(context.Context, *sqs.ReceiveMessageInput, ...request.Option) (*sqs.ReceiveMessageOutput, error)
//   m.On("ReceiveMessageWithContext", awsmock.AnyContext, input, awsmock.NilOpts)
var AnyContext = mock.MatchedBy(func(ctx context.Context) bool {
	return true
})
