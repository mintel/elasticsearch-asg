package cmd

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty" // Check if running in a terminal.
	"go.uber.org/zap"            // Logging.
	"go.uber.org/zap/zapcore"
)

// LoggingFlags represents a set of flags for setting up logging.
type LoggingFlags struct {
	LogLevel zapcore.Level // Logging level.
}

// NewLoggingFlags returns a new LoggingFlags.
func NewLoggingFlags(app Flagger, logLevel string) *LoggingFlags {
	var f LoggingFlags

	app.Flag("log.level", "Set logging level.").
		HintOptions(
			zap.DebugLevel.CapitalString(),
			zap.DebugLevel.String(),
			zap.InfoLevel.CapitalString(),
			zap.InfoLevel.String(),
			zap.WarnLevel.CapitalString(),
			zap.WarnLevel.String(),
			zap.ErrorLevel.CapitalString(),
			zap.ErrorLevel.String(),
			zap.DPanicLevel.CapitalString(),
			zap.DPanicLevel.String(),
			zap.PanicLevel.CapitalString(),
			zap.PanicLevel.String(),
			zap.FatalLevel.CapitalString(),
			zap.FatalLevel.String(),
		).
		Default(logLevel).
		SetValue(&f.LogLevel)

	return &f
}

// NewLogger returns a new logger based on the LogLevel flag.
func (f *LoggingFlags) NewLogger() *zap.Logger {
	var conf zap.Config

	// If program is running in a terminal, use the zap default
	// dev logging config, else prod logging config.
	if isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		conf = zap.NewDevelopmentConfig()
	} else {
		conf = zap.NewProductionConfig()
	}

	conf.Level.SetLevel(f.LogLevel)

	logger, err := conf.Build()
	if err != nil {
		panic(fmt.Sprintf("error building logger: %s", err))
	}

	return logger
}
