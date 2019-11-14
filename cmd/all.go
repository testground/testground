package cmd

import (
	"sync"

	"github.com/ipfs/testground/pkg/engine"
	"github.com/urfave/cli"
)

// _engine is the default engine shared by all commands.
var (
	_engine     *engine.Engine
	_engineErr  error
	_engineOnce sync.Once
)

func GetEngine() (*engine.Engine, error) {
	_engineOnce.Do(func() {
		_engine, _engineErr = engine.NewDefaultEngine()
	})
	return _engine, _engineErr
}

// Commands collects all subcommands of the testground CLI.
var Commands = []cli.Command{
	RunCommand,
	ListCommand,
	BuildCommand,
	DescribeCommand,
	SidecarCommand,
}
