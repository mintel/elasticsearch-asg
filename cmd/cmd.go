package cmd

import (
	"os"

	"github.com/mattn/go-isatty"             // Check if program is running is a terminal
	"go.uber.org/zap"                        // Logging
	kingpin "gopkg.in/alecthomas/kingpin.v2" // Command line args parser
)

var verboseFlag = kingpin.Flag("verbose", "Show debug logging.").Short('v').Bool()

// SetupLogging sets up zap logging.
func SetupLogging() *zap.Logger {
	var conf zap.Config
	// If program is running in a terminal, use the zap default dev logging config, else prod logging config.
	if isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		conf = zap.NewDevelopmentConfig()
	} else {
		conf = zap.NewProductionConfig()
	}
	// If the --verbose flag is set, log at the debug level.
	if *verboseFlag {
		conf.Level.SetLevel(zap.DebugLevel)
	}
	logger, err := conf.Build() // Convert logging config to logger.
	if err != nil {
		panic(err)
	}
	_ = zap.ReplaceGlobals(logger) // Set global zap logger. `zap.L()` will return this logger.
	_ = zap.RedirectStdLog(logger) // Redirect stdlib logger (`log` package) to zap logger.
	return logger
}
