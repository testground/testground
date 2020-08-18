package cmd

import (
	"context"
	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/client"
	"github.com/urfave/cli/v2"
)

var TasksCommand = cli.Command{
	Name:   "tasks",
	Usage:  "get a list of the existing tasks",
	Action: tasksCommand,
	Flags: []cli.Flag{
		// TODO: filters
	},
}

func tasksCommand(c *cli.Context) error {
	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	cl, _, err := setupClient(c)
	if err != nil {
		return err
	}

	r, err := cl.Tasks(ctx, &api.TasksRequest{

	})
	if err != nil {
		return err
	}
	defer r.Close()

	_, err = client.ParseTasksRequest(r)
	// TODO: parse response
	return err
}
