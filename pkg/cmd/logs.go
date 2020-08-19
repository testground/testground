package cmd

import (
	"context"
	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/client"
	"github.com/urfave/cli/v2"
)

var LogsCommand = cli.Command{
	Name:   "logs",
	Usage:  "get the current status for a certain task",
	Action: logsCommand,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "task",
			Aliases:  []string{"t"},
			Usage:    "the task id",
			Required: true,
		},
		&cli.BoolFlag{
			Name:    "follow",
			Aliases: []string{"f"},
			Usage:   "stream the logs until the task completes",
		},
	},
}

func logsCommand(c *cli.Context) error {
	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	cl, _, err := setupClient(c)
	if err != nil {
		return err
	}

	r, err := cl.Logs(ctx, &api.LogsRequest{
		TaskID: c.String("task"),
		Follow: c.Bool("follow"),
	})
	if err != nil {
		return err
	}
	defer r.Close()

	tsk, err := client.ParseLogsRequest(r)
	if err != nil {
		return err
	}

	printTask(tsk)
	return nil
}
