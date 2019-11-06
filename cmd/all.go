package cmd

import (
	"github.com/ipfs/testground/pkg/engine"
	"github.com/urfave/cli"
)

// _engine is the default engine shared by all commands.
var _engine *engine.Engine = func() *engine.Engine {
	e, err := engine.NewDefaultEngine()
	if err != nil {
		panic(err)
	}
	return e
}()

// Commands collects all subcommands of the testground CLI.
var Commands = []cli.Command{
	RunCommand,
	ListCommand,
	BuildCommand,
}
