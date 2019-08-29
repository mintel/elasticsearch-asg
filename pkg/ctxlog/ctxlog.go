// Package ctxlog provides functionality for adding/extracting a zap Logger
// to/from a Context object.
package ctxlog

import (
	"context"

	"go.uber.org/zap"
)

type (
	loggerKeyType        struct{}
	sugaredLoggerKeyType struct{}
)

var (
	// loggerKey is a unique key to embed a zap.Logger in a Context.
	loggerKey = loggerKeyType{}

	// sugaredLoggerKey is a unique key to embed a zap.SugaredLogger in a Context.
	sugaredLoggerKey = sugaredLoggerKeyType{}

	// nop logger to ensure FromContext always returns something
	nop = zap.NewNop()

	// L is an alias for GetLogger.
	L = GetLogger

	// S is an alias for GetSugaredLogger.
	S = GetSugaredLogger
)

// WithLogger embeds Logger in the given Context. Later the logger can be
// obtained by GetLogger or GetSugaredLogger.
func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	// There is a performance difference depending on the order you
	// attach multiple values to a context.
	// Attach the SugaredLogger first since it's used less often.
	ctx = context.WithValue(ctx, sugaredLoggerKey, logger.Sugar())
	ctx = context.WithValue(ctx, loggerKey, logger)
	return ctx
}

// WithFields adds the given fields to the Logger embedded in ctx.
func WithFields(ctx context.Context, fields ...zap.Field) context.Context {
	return WithLogger(ctx, GetLogger(ctx).With(fields...))
}

// WithName adds the given name to the Logger embedded in ctx.
func WithName(ctx context.Context, name string) context.Context {
	return WithLogger(ctx, GetLogger(ctx).Named(name))
}

// GetLogger either returns an embedded Logger from the context
// or a nop Logger if nothing is embedded.
func GetLogger(ctx context.Context) *zap.Logger {
	l := ctx.Value(loggerKey)
	if l == nil {
		return nop
	}
	return l.(*zap.Logger)
}

// GetSugaredLogger either returns an embedded SugaredLogger from the context
// or a nop Logger if nothing is embedded.
func GetSugaredLogger(ctx context.Context) *zap.SugaredLogger {
	l := ctx.Value(sugaredLoggerKey)
	if l == nil {
		return nop.Sugar()
	}
	return l.(*zap.SugaredLogger)
}
