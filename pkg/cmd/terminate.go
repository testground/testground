package cmd

import (
	"context"
	"errors"

	"github.com/urfave/cli/v2"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/client"
)

var TerminateCommand = cli.Command{
	Name:   "terminate",
	Usage:  "terminate all jobs and supporting processes of a runner",
	Action: terminateCommand,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "runner",
			Usage: "runner to terminate; values include: 'local:exec', 'local:docker', 'cluster:k8s'",
		},
		&cli.StringFlag{
			Name:  "builder",
			Usage: "builder to terminate; values include: 'docker:go', 'docker:generic', 'exec:go'",
		},
	},
}

func terminateCommand(c *cli.Context) error {
	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	var (
		runner  = c.String("runner")
		builder = c.String("builder")
	)

	if runner != "" && builder != "" {
		return errors.New("cannot accept runner and builder at the same time; please do one at a time")
	}

	if runner == "" && builder == "" {
		return errors.New("specify something to terminate")
	}

	cl, _, err := setupClient(c)
	if err != nil {
		return err
	}

	r, err := cl.Terminate(ctx, &api.TerminateRequest{
		Runner:  runner,
		Builder: builder,
	})
	if err != nil {
		return err
	}
	defer r.Close()

	return client.ParseTerminateRequest(r)
}
