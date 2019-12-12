package cmd

import (
	"context"
	"fmt"

	"github.com/ipfs/testground/pkg/config"
	"github.com/ipfs/testground/pkg/daemon/client"
	"github.com/ipfs/testground/pkg/server"
	"github.com/urfave/cli"
)

// ListCommand is the specification of the `list` command.
var ListCommand = cli.Command{
	Name:   "list",
	Usage:  "list all test plans and test cases",
	Action: listCommand,
}

func listCommand(ctx *cli.Context) error {
	api, cancel, err := setupClient()
	if err != nil {
		return err
	}
	defer cancel()

	resp, err := api.List(context.Background())
	if err != nil {
		return fmt.Errorf("fatal error from daemon: %s", err)
	}
	defer resp.Close()

	return client.ProcessListResponse(resp)
}

func setupClient() (*client.Client, func(), error) {
	cancel := func() {}

	envcfg, err := config.GetEnvConfig()
	if err != nil {
		return nil, cancel, err
	}
	if envcfg.Client.Endpoint == "" {
		var ctx context.Context
		ctx, cancel = context.WithCancel(context.Background())

		envcfg.Client.Endpoint, err = server.ListenAndServe(ctx)
		if err != nil {
			return nil, cancel, err
		}
	}

	api := client.New(envcfg.Client.Endpoint)

	return api, cancel, nil
}
