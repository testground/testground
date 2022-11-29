package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/client"
	"github.com/testground/testground/pkg/data"
	"github.com/testground/testground/pkg/task"
	"github.com/urfave/cli/v2"
)

var StatusCommand = cli.Command{
	Name:   "status",
	Usage:  "get the current status for a certain task",
	Action: statusCommand,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "extended",
			Usage: "print extended information such as input and results",
		},
		&cli.StringFlag{
			Name:     "task",
			Aliases:  []string{"t"},
			Usage:    "the task id",
			Required: true,
		},
	},
}

func statusCommand(c *cli.Context) error {
	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	id := c.String("task")

	cl, _, err := setupClient(c)
	if err != nil {
		return err
	}

	r, err := cl.Status(ctx, &api.StatusRequest{TaskID: id})
	if err != nil {
		return err
	}
	defer r.Close()

	res, err := client.ParseStatusResponse(r, c.App.Writer)
	if err != nil {
		return err
	}

	printTask(res)

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

func printTask(tsk task.Task) {
	outcome, err := data.DecodeTaskOutcome(&tsk)
	outcomeStr := string(outcome)

	if err != nil {
		outcomeStr = fmt.Sprintf("failed to decode task outcome: %v", err)
	}

	fmt.Printf("ID:\t\t%s\n", tsk.ID)
	fmt.Printf("Priority:\t%d\n", tsk.Priority)
	fmt.Printf("Created:\t%s\n", tsk.Created())
	fmt.Printf("Type:\t\t%s\n", tsk.Type)
	fmt.Printf("Status:\t\t%s\n", tsk.State().State)
	fmt.Printf("Outcome:\t%s\n", outcomeStr)
	fmt.Printf("Last update:\t%s\n", tsk.State().Created)
}
