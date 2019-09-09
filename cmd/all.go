package cmd

import (
	"github.com/ipfs/testground/pkg/engine"
	"github.com/urfave/cli"
)

var Engine = engine.NewDefaultEngine()

// Commands collects all subcommands of the testground CLI.
var Commands = []cli.Command{
	RunCommand,
	ListCommand,
	BuildCommand,
}
