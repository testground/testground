package cmd

import (
	"fmt"
	"strings"

	"errors"

	"github.com/urfave/cli"
)

// RunCommand is the definition of the `run` command.
var RunCommand = cli.Command{
	Name:      "run",
	Usage:     "run test case with name `testplan/testcase`",
	Action:    runCommand,
	ArgsUsage: "[name]",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "runner, r",
			Value: "local",
			Usage: "specifies the runner; options: [local, nomad]",
		},
		cli.StringFlag{
			Name:  "nomad-api, n",
			Value: "http://127.0.0.1:5000",
			Usage: "nomad endpoint",
		},
	},
}

func runCommand(c *cli.Context) error {
	if c.NArg() != 1 {
		cli.ShowSubcommandHelp(c)
		fmt.Println("")
		return errors.New("missing test name")
	}
	testcase := c.Args().First()
	if testcase == "" {

	}
	comp := strings.Split(testcase, "/")
	if len(comp) != 2 {
		cli.ShowSubcommandHelp(c)
		return errors.New("wrong format for test case name, should be: `testplan/testcase`")
	}
	return nil
}
