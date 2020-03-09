package cmd

import (
	"context"

	"github.com/ipfs/testground/pkg/client"
	"github.com/urfave/cli"
)

var TerminateCommand = cli.Command{
	Name:   "terminate",
	Usage:  " terminates all jobs running on a runner",
	Action: terminateCommand,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:     "runner",
			Usage:    "specifies the runner to use; values include: 'local:exec', 'local:docker', 'cluster:k8s'",
			Required: true,
		},
	},
}

func terminateCommand(c *cli.Context) error {
	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	runner := c.String("runner")

	api, err := setupClient(c)
	if err != nil {
		return err
	}

	r, err := api.Terminate(ctx, &client.TerminateRequest{
		Runner: runner,
	})
	if err != nil {
		return err
	}
	defer r.Close()

	return client.ParseTerminateRequest(r)
}
