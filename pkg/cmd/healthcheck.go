package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/client"
)

var HealthcheckCommand = cli.Command{
	Name:   "healthcheck",
	Usage:  "checks, and optionally heals, the preconditions for the runner to be able to run properly",
	Action: healthcheckCommand,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "fix",
			Usage: "should try to fix the preconditions",
		},
		cli.StringFlag{
			Name:     "runner",
			Usage:    "specifies the runner to use; values include: 'local:exec', 'local:docker', 'cluster:k8s'",
			Required: true,
		},
	},
}

func healthcheckCommand(c *cli.Context) error {
	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	var (
		runner = c.String("runner")
		fix    = c.Bool("fix")
	)

	cl, _, err := setupClient(c)
	if err != nil {
		return err
	}

	r, err := cl.Healthcheck(ctx, &api.HealthcheckRequest{
		Runner: runner,
		Fix:    fix,
	})
	if err != nil {
		return err
	}
	defer r.Close()

	resp, err := client.ParseHealthcheckResponse(r)
	if err != nil {
		return err
	}

	fmt.Printf("finished checking runner %s\n", runner)
	fmt.Println(resp.String())

	return nil
}
