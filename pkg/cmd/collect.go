package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/client"
	"github.com/testground/testground/pkg/logging"

	"github.com/urfave/cli/v2"
)

// CollectCommand is the specification of the `collect` command.
var CollectCommand = cli.Command{
	Name:      "collect",
	Usage:     "Produces a tgz file with the output from a certain run",
	Action:    collectCommand,
	ArgsUsage: "[run_id]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "runner",
			Aliases:  []string{"r"},
			Usage:    "specifies the runner to use; values include: 'local:exec', 'local:docker', 'cluster:k8s'",
			Required: true,
		},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "specifies a named output for the tgz file",
		},
	},
}

func collectCommand(c *cli.Context) error {
	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	if c.NArg() != 1 {
		return errors.New("missing run id")
	}

	var (
		id     = c.Args().First()
		runner = c.String("runner")
		output = id + ".tgz"
	)

	if o := c.String("output"); o != "" {
		output = o
	}

	cl, _, err := setupClient(c)
	if err != nil {
		return err
	}

	return collect(ctx, cl, runner, id, output)
}

func collect(ctx context.Context, cl *client.Client, runner string, runid string, outputFile string) error {
	req := &api.OutputsRequest{
		Runner: runner,
		RunID:  runid,
	}

	resp, err := cl.CollectOutputs(ctx, req)
	if err != nil {
		if err == context.Canceled {
			return fmt.Errorf("interrupted")
		}
		return err
	}
	defer resp.Close()

	file, err := os.Create(outputFile)
	if err != nil {
		if err == context.Canceled {
			return fmt.Errorf("interrupted")
		}
		return err
	}
	defer file.Close()

	cr, err := client.ParseCollectResponse(resp, file)
	if err != nil {
		return err
	}

	if !cr.Exists {
		logging.S().Errorw("no such testplan run", "run_id", runid, "runner", runner)

		return os.Remove(outputFile)
	}

	logging.S().Infof("created file: %s", outputFile)
	return nil
}
