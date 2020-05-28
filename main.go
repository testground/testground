package main

import (
	"fmt"
	"os"

	"github.com/testground/testground/pkg/cmd"
	"github.com/testground/testground/pkg/logging"

	"github.com/urfave/cli/v2"
	"go.uber.org/zap/zapcore"
)

func main() {
	app := cli.NewApp()
	app.Name = "testground"
	app.Usage = "a platform for testing, benchmarking, and simulating distributed and p2p systems at scale"
	app.Description = "testground is a platform for testing, benchmarking, " +
		"and simulating distributed and p2p systems at scale."
	app.Commands = cmd.RootCommands
	app.Flags = cmd.RootFlags
	// Disable the built-in -v flag (version), to avoid collisions with the
	// verbosity flags.
	// TODO implement a `testground version` command instead.
	app.HideVersion = true
	app.Before = func(c *cli.Context) error {
		configureLogging(c)
		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func configureLogging(c *cli.Context) {
	// The LOG_LEVEL environment variable takes precedence.
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		var l zapcore.Level
		if err := l.UnmarshalText([]byte(level)); err != nil {
			panic(err)
		}
		logging.SetLevel(l)
		return
	}

	// Apply verbosity flags.
	switch {
	case c.Bool("v"):
		logging.SetLevel(zapcore.DebugLevel)
	case c.Bool("vv"):
		logging.SetLevel(zapcore.DebugLevel)
	default:
		// Do nothing; level remains at default (INFO).
	}
}
