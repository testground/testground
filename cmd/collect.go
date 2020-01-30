package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ipfs/testground/pkg/daemon/client"
	"github.com/urfave/cli"
)

// CollectCommand is the specification of the `collect` command.
var CollectCommand = cli.Command{
	Name:      "collect",
	Usage:     "TODO",
	Action:    collectCommand,
	ArgsUsage: "[run-id]",
	Flags: []cli.Flag{
		cli.GenericFlag{
			Name: "runner, r",
			Value: &EnumValue{
				Allowed: runners,
			},
			Usage: fmt.Sprintf("specifies the runner; options: %s", strings.Join(runners, ", ")),
		},
	},
}

func collectCommand(c *cli.Context) error {
	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	if c.NArg() != 1 {
		_ = cli.ShowSubcommandHelp(c)
		return errors.New("missing run id")
	}

	var (
		runID    = c.Args().First()
		runnerID = c.Generic("runner").(*EnumValue).String()
	)

	api, err := setupClient(c)
	if err != nil {
		return err
	}

	req := &client.OutputsRequest{
		Runner: runnerID,
		Run:    runID,
	}

	resp, err := api.CollectOutputs(ctx, req)
	if err != nil {
		if err == context.Canceled {
			return fmt.Errorf("interrupted")
		}
		return fmt.Errorf("fatal error from daemon: %s", err)
	}
	defer resp.Close()

	// TODO: resp to file

	return nil
}
