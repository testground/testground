package cmd

import (
	"context"
	"fmt"

	"github.com/ipfs/testground/pkg/daemon/client"
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

	api, err := setupClient()
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
