package cmd

import "github.com/urfave/cli/v2"

// RootCommands collects all subcommands of the testground CLI.
var RootCommands = cli.Commands{
	&RunCommand,
	&PlanCommand,
	&BuildCommand,
	&DescribeCommand,
	&SidecarCommand,
	&DaemonCommand,
	&CollectCommand,
	&TerminateCommand,
	&HealthcheckCommand,
}

var RootFlags = []cli.Flag{
	&cli.BoolFlag{
		Name:  "v",
		Usage: "verbose output (equivalent to DEBUG log level)",
	},
	&cli.BoolFlag{
		Name:  "vv",
		Usage: "super verbose output (equivalent to DEBUG log level for now, it may accommodate TRACE in the future)",
	},
	&cli.StringFlag{
		Name:  "endpoint",
		Usage: "set the daemon endpoint URI (overrides .env.toml)",
	},
}
