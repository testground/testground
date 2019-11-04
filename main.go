package main

import (
	"fmt"
	"os"

	"github.com/ipfs/testground/pkg/logging"
	"go.uber.org/zap/zapcore"

	"github.com/ipfs/testground/cmd"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "testground"
	app.Commands = cmd.Commands
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "v",
			Usage: "verbose output (equivalent to INFO log level)",
		},
		cli.BoolFlag{
			Name:  "vv",
			Usage: "super verbose output (equivalent to DEBUG log level)",
		},
	}
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
	}
}

func configureLogging(c *cli.Context) {
	logging.TerminalMode()

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
		logging.SetLevel(zapcore.InfoLevel)
	case c.Bool("vv"):
		logging.SetLevel(zapcore.DebugLevel)
	default:
		// Do nothing; level remains at default (WARN).
	}
}
