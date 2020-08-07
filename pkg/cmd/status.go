package cmd

import (
	"context"
	"errors"
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

	r, err := cl.TaskInfo(ctx, id)
	if err != nil {
		return err
	}
	defer r.Close()

	_, err = client.ParseTaskInfoResponse(r)

	// bytes, err := json.MarshalIndent(tsk, "", "\t")
	// if err != nil {
	return err
}
