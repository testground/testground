package cmd

import (
	"github.com/ipfs/testground/pkg/config"
	"github.com/ipfs/testground/pkg/daemon/client"
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

var Flags = []cli.Flag{
	cli.BoolFlag{
		Name:  "v",
		Usage: "verbose output (equivalent to DEBUG log level)",
	},
	cli.BoolFlag{
		Name:  "vv",
		Usage: "super verbose output (equivalent to DEBUG log level for now, it may accommodate TRACE in the future)",
	},
	cli.StringFlag{
		Name:  "endpoint",
		Usage: "set the daemon endpoint URI (overrides .env.toml)",
	},
}

func setupClient(c *cli.Context) (*client.Client, error) {
	endpoint := c.Parent().String("endpoint")

	if endpoint == "" {
		envcfg, err := config.GetEnvConfig()
		if err != nil {
			return nil, err
		}
		endpoint = envcfg.Client.Endpoint
	}

	api := client.New(endpoint)
	return api, nil
}
