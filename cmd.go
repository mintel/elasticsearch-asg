package esasg

import (
	"os"

	"github.com/mattn/go-isatty"
	"go.uber.org/zap"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var verboseFlag = kingpin.Flag("verbose", "Show debug logging.").Short('v').Bool()

// SetupLogging sets up zap logging.
func SetupLogging() *zap.Logger {
	var conf zap.Config
	if isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		conf = zap.NewDevelopmentConfig()
	} else {
		conf = zap.NewProductionConfig()
	}
	if *verboseFlag {
		conf.Level.SetLevel(zap.DebugLevel)
	}
	logger, err := conf.Build()
	if err != nil {
		panic(err)
	}
	_ = zap.ReplaceGlobals(logger)
	_ = zap.RedirectStdLog(logger)
	return logger
}
