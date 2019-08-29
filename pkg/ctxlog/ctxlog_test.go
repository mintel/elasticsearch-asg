package ctxlog

import (
	"context"
	"testing"

	"go.uber.org/zap/zaptest/observer"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/stretchr/testify/assert"
)

func TestWithLoggerGetLogger(t *testing.T) {
	ctx := context.Background()

	t.Run("nop", func(t *testing.T) {
		got := L(ctx)
		want := zapcore.NewNopCore()
		assert.Equal(t, want, got.Core())
	})

	want, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	ctx = WithLogger(ctx, want)

	t.Run("logger", func(t *testing.T) {
		got := L(ctx)
		assert.Equal(t, want, got)
	})

	t.Run("sugar", func(t *testing.T) {
		got := S(ctx)
		assert.Equal(t, want.Sugar(), got)
	})
}

func TestWithFields(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	field := zap.String("foo", "bar")
	logger := zap.New(core)
	logger.Debug("no fields")
	ctx := WithLogger(context.Background(), logger)
	ctx = WithFields(ctx, field)
	L(ctx).Debug("with field")
	assert.Equal(t, 2, logs.Len(), "wrong number of log entries")
	assert.Equal(t, 1, logs.FilterField(field).Len(), "log entry with field is missing")
}

func TestWithName(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	const name = "foobar"
	logger := zap.New(core)
	logger.Debug("no name")
	ctx := WithLogger(context.Background(), logger)
	ctx = WithName(ctx, name)
	L(ctx).Debug("with name")
	assert.Equal(t, 2, logs.Len(), "wrong number of log entries")
	assert.Equal(t, name, logs.All()[1].Entry.LoggerName, "logger entry has wrong name")
}
