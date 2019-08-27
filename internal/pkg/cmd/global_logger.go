package cmd

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// SetGlobalLogger both sets the zap global logger, and
// redirects the output from the standard library's
// package-global logger to the supplied logger at the debug level.
// It returns a teardown function to reset the global loggers.
func SetGlobalLogger(logger *zap.Logger) func() {
	return SetGlobalLoggerAt(logger, zap.DebugLevel)
}

// SetGlobalLogger both sets the zap global logger, and
// redirects the output from the standard library's
// package-global logger to the supplied logger at the specified level.
// It returns a teardown function to reset the global loggers.
func SetGlobalLoggerAt(logger *zap.Logger, stdLogLevel zapcore.Level) func() {
	t1 := zap.ReplaceGlobals(logger)
	t2, err := zap.RedirectStdLogAt(logger, stdLogLevel)
	if err != nil {
		panic(err)
	}
	return func() {
		t2()
		t1()
	}
}
