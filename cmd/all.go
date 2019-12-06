package cmd

import (
	"github.com/urfave/cli"
)

// Commands collects all subcommands of the testground CLI.
var Commands = []cli.Command{
	RunCommand,
	ListCommand,
	BuildCommand,
	DescribeCommand,
	SidecarCommand,
	DaemonCommand,
}
