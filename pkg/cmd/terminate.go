package cmd

import (
	"context"

	"github.com/urfave/cli/v2"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/client"
)

var TerminateCommand = cli.Command{
	Name:   "terminate",
	Usage:  "terminates all jobs running on a runner",
	Action: terminateCommand,
	Flags: []cli.Flag{
		&cli.StringFlag{
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

	cl, _, err := setupClient(c)
	if err != nil {
		return err
	}

	r, err := cl.Terminate(ctx, &api.TerminateRequest{
		Runner: runner,
	})
	if err != nil {
		return err
	}
	defer r.Close()

	return client.ParseTerminateRequest(r)
}
