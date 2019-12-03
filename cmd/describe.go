package cmd

import (
	"context"
	"errors"

	"github.com/ipfs/testground/pkg/daemon/client"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/pkg/server"
	"github.com/urfave/cli"
)

// DescribeCommand is the specification of the `list` command.
var DescribeCommand = cli.Command{
	Name:      "describe",
	Usage:     "describes a test plan or test case",
	ArgsUsage: "[term], where " + server.TermExplanation,
	Action:    describeCommand,
}

func describeCommand(ctx *cli.Context) error {
	if ctx.NArg() == 0 {
		_ = cli.ShowSubcommandHelp(ctx)
		return errors.New("missing term to describe; " + server.TermExplanation)
	}

	term := ctx.Args().First()

	api, cancel, err := setupClient()
	if err != nil {
		return err
	}
	defer cancel()

	req := &client.DescribeRequest{
		Term: term,
	}
	resp, err := api.Describe(context.Background(), req)
	if err != nil {
		logging.S().Fatal(err.Error())
	}
	defer resp.Close()

	return client.ProcessDescribeResponse(resp)
}
