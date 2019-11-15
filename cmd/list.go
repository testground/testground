package cmd

import (
	"fmt"

	"github.com/urfave/cli"
)

// ListCommand is the specification of the `list` command.
var ListCommand = cli.Command{
	Name:   "list",
	Usage:  "list all test plans and test cases",
	Action: listCommand,
}

func listCommand(c *cli.Context) error {
	engine, err := GetEngine()
	if err != nil {
		return err
	}
	plans := engine.TestCensus().ListPlans()
	for _, tp := range plans {
		for _, c := range tp.TestCases {
			fmt.Println(tp.Name + "/" + c.Name)
		}
	}
	return nil
}
