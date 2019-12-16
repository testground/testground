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

func setupClient() (*client.Client, error) {
	envcfg, err := config.GetEnvConfig()
	if err != nil {
		return nil, err
	}

	api := client.New(envcfg.Client.Endpoint)
	return api, nil
}
