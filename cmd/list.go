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

func listCommand(c *cli.Context) error {
	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	api, err := setupClient(ctx)
	if err != nil {
		return err
	}

	resp, err := api.List(ctx)
	if err != nil {
		return fmt.Errorf("fatal error from daemon: %s", err)
	}
	defer resp.Close()

	return client.ParseListResponse(resp)
}

func setupClient(ctx context.Context) (*client.Client, error) {
	envcfg, err := config.GetEnvConfig()
	if err != nil {
		return nil, err
	}
	if envcfg.Client.Endpoint == "" {
		envcfg.Client.Endpoint, err = server.ListenAndServe(ctx)
		if err != nil {
			return nil, err
		}
	}

	api := client.New(envcfg.Client.Endpoint)

	return api, nil
}
