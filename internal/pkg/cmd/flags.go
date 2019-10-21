package cmd

import (
	kingpin "gopkg.in/alecthomas/kingpin.v2" // Command line flag parsing.
)

// Flagger defines command line flags and args.
// Examples: kingpin.Application and kingping.CmdClause.
type Flagger interface {
	Flag(name string, help string) *kingpin.FlagClause
	Arg(name string, help string) *kingpin.ArgClause
}

// Assert the Flagger interface matches the things
// it needs to match.
var (
	_ Flagger = (*kingpin.Application)(nil)
	_ Flagger = (*kingpin.CmdClause)(nil)
)
