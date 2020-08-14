package cmd

import (
	"context"
	"errors"
	"fmt"
	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/client"
	"github.com/urfave/cli/v2"
)

var StatusCommand = cli.Command{
	Name:      "status",
	Usage:     "get the current status for a certain task",
	Action:    statusCommand,
	ArgsUsage: "[task_id]",
	Flags:     []cli.Flag{},
}

func statusCommand(c *cli.Context) error {
	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	if c.NArg() != 1 {
		return errors.New("missing run id")
	}

	var (
		id = c.Args().First()
	)

	cl, _, err := setupClient(c)
	if err != nil {
		return err
	}

	r, err := cl.TaskStatus(ctx, &api.TaskStatusRequest{ID: id})
	if err != nil {
		return err
	}
	defer r.Close()

	res, err := client.ParseTaskStatusResponse(r)
	if err != nil {
		return err
	}

	fmt.Printf("ID:\t\t%s\n", res.ID)
	fmt.Printf("Created:\t%s\n", res.Created)
	fmt.Printf("Type:\t\t%s\n", res.Type)
	fmt.Printf("Status:\t\t%s\n", res.LastState)
	fmt.Printf("Last update:\t%s\n", res.LastUpdate)

	return nil
}
