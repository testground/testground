package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ipfs/testground/pkg/daemon/client"
	"github.com/urfave/cli"
)

// CollectCommand is the specification of the `collect` command.
var CollectCommand = cli.Command{
	Name:      "collect",
	Usage:     "Produces a zip file with the output from a certain run",
	Action:    collectCommand,
	ArgsUsage: "[run-id]",
	Flags: []cli.Flag{
		cli.GenericFlag{
			Name:     "runner, r",
			Required: true,
			Value: &EnumValue{
				Allowed: runners,
			},
			Usage: fmt.Sprintf("specifies the runner; options: %s", strings.Join(runners, ", ")),
		},
		cli.StringFlag{
			Name:  "output, o",
			Usage: "specifies a named output for the zip file",
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
		output   = runID + ".zip"
	)

	if o := c.String("output"); o != "" {
		output = o
	}

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

	file, err := os.Create(output)
	if err != nil {
		if err == context.Canceled {
			return fmt.Errorf("interrupted")
		}
		return fmt.Errorf("fatal error from daemon: %s", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp)
	return err
}
