package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ipfs/testground/pkg/client"
	"github.com/ipfs/testground/pkg/logging"

	"github.com/urfave/cli"
)

// CollectCommand is the specification of the `collect` command.
var CollectCommand = cli.Command{
	Name:      "collect",
	Usage:     "Produces a zip file with the output from a certain run",
	Action:    collectCommand,
	ArgsUsage: "[run_id]",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:     "runner, r",
			Usage:    "specifies the runner to use; values include: 'local:exec', 'local:docker', 'cluster:k8s'",
			Required: true,
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
		id     = c.Args().First()
		runner = c.String("runner")
		output = id + ".zip"
	)

	if o := c.String("output"); o != "" {
		output = o
	}

	api, err := setupClient(c)
	if err != nil {
		return err
	}

	req := &client.OutputsRequest{
		Runner: runner,
		RunID:  id,
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
	if err != nil {
		return err
	}

	logging.S().Infof("created file: %s", output)

	return nil
}
