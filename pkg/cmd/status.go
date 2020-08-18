package cmd

import (
	"context"
	"encoding/json"
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
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "extended",
			Usage: "print extended information such as input and results",
		},
	},
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

	r, err := cl.Status(ctx, &api.StatusRequest{TaskID: id})
	if err != nil {
		return err
	}
	defer r.Close()

	res, err := client.ParseStatusResponse(r)
	if err != nil {
		return err
	}

	fmt.Printf("ID:\t\t%s\n", res.ID)
	fmt.Printf("Priority:\t%d\n", res.Priority)
	fmt.Printf("Created:\t%s\n", res.Created())
	fmt.Printf("Type:\t\t%s\n", res.Type)
	fmt.Printf("Status:\t\t%s\n", res.State().TaskState)
	fmt.Printf("Last update:\t%s\n", res.State().Created)

	if c.Bool("extended") {
		fmt.Printf("\nInput:\n")
		input, err := json.Marshal(res.Input)
		if err != nil {
			return err
		}
		fmt.Println(string(input))

		fmt.Printf("\nResult:\n")
		output, err := json.Marshal(res.Result)
		if err != nil {
			return err
		}
		fmt.Println(string(output))
	}

	return nil
}
