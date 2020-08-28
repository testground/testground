package cmd

import (
	"context"
	"fmt"
	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/client"
	"github.com/testground/testground/pkg/task"
	"github.com/urfave/cli/v2"
	"os"
	"text/tabwriter"
)

var TasksCommand = cli.Command{
	Name:   "tasks",
	Usage:  "get a list of the existing tasks",
	Action: tasksCommand,
	Flags: []cli.Flag{
		// TODO(hac): filters
	},
}

func tasksCommand(c *cli.Context) error {
	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	cl, _, err := setupClient(c)
	if err != nil {
		return err
	}

	req := &api.TasksRequest{
		Types:  []task.Type{task.TypeBuild, task.TypeRun},
		States: []task.State{task.StateScheduled, task.StateProcessing, task.StateComplete},
	}

	r, err := cl.Tasks(ctx, req)
	if err != nil {
		return err
	}
	defer r.Close()

	tsks, err := client.ParseTasksRequest(r)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)

	fmt.Fprintln(w, "ID\tState\tType")

	for _, tsk := range tsks {
		fmt.Fprintf(w, "%s\t%s\t%s\n", tsk.ID, tsk.State().State, tsk.Type)
	}

	w.Flush()

	return err
}
