package squeues

import (
	"context"

	"github.com/aws/aws-sdk-go/service/sqs" // AWS SQS client
)

// Handler is an interface for handling messages received from an SQS queue.
// It's passed into SQS.Run().
type Handler interface {
	Handle(context.Context, *sqs.Message) error
}

// funcHandler is a Handler which wraps a function.
type funcHandler struct {
	Fn func(context.Context, *sqs.Message) error
}

func (h *funcHandler) Handle(ctx context.Context, m *sqs.Message) error {
	return h.Fn(ctx, m)
}

var _ Handler = (*funcHandler)(nil) // Assert funcHandler implements the Handler interface.

// FuncHandler returns a simple Handler wrapper around a function.
func FuncHandler(fn func(context.Context, *sqs.Message) error) Handler {
	return &funcHandler{
		Fn: fn,
	}
}
