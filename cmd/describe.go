package cmd

import (
	"context"
	"errors"

	"github.com/ipfs/testground/pkg/client"
	"github.com/ipfs/testground/pkg/daemon"
	"github.com/urfave/cli"
)

// DescribeCommand is the specification of the `list` command.
var DescribeCommand = cli.Command{
	Name:      "describe",
	Usage:     "describes a test plan or test case",
	ArgsUsage: "[term], where " + daemon.TermExplanation,
	Action:    describeCommand,
}

func describeCommand(c *cli.Context) error {
	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	if c.NArg() == 0 {
		_ = cli.ShowSubcommandHelp(c)
		return errors.New("missing term to describe; " + daemon.TermExplanation)
	}

	term := c.Args().First()

	api, err := setupClient(c)
	if err != nil {
		return err
	}

	req := &client.DescribeRequest{
		Term: term,
	}
	resp, err := api.Describe(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Close()

	return client.ParseDescribeResponse(resp)
}
