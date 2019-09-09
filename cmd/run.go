package cmd

import (
	"github.com/urfave/cli"
)

// RunCommand is the definition of the `run` command.
var RunCommand = cli.Command{
	Name:      "run",
	Usage:     "run test case with name `testplan/testcase`",
	Action:    runCommand,
	ArgsUsage: "[name]",
	Flags: []cli.Flag{
		cli.GenericFlag{
			Name: "runner, r",
			Value: &EnumValue{
				Allowed: []string{"local", "nomad"},
				Default: "local",
			},
			Usage: "specifies the runner; options: [local, nomad]",
		},
		cli.StringFlag{
			Name:  "nomad-api, n",
			Value: "http://127.0.0.1:5000",
			Usage: "nomad endpoint",
		},
		cli.IntFlag{
			Name:  "instances, i",
			Value: 10,
			Usage: "number of instances of the test case to run",
		},
	},
}

func runCommand(c *cli.Context) error {
	// if c.NArg() != 1 {
	// 	cli.ShowSubcommandHelp(c)
	// 	fmt.Println("")
	// 	return errors.New("missing test name")
	// }

	// testcase := c.Args().First()
	// if testcase == "" {
	// 	cli.ShowSubcommandHelp(c)
	// 	return errors.New("no test case provided; use the `list` command to view available test cases")
	// }

	// comp := strings.Split(testcase, "/")
	// if len(comp) != 2 {
	// 	cli.ShowSubcommandHelp(c)
	// 	return errors.New("wrong format for test case name, should be: `testplan/testcase`")
	// }

	// tp := api.TestCensus().ByName(comp[0])
	// if tp == nil {
	// 	return errors.New("unrecognized test plan; use the `list` command to view available test plans and cases")
	// }

	// seq, _, ok := tp.TestCaseByName(comp[1])
	// if !ok {
	// 	return errors.New("unrecognized test case; use the `list` command to view available test cases")
	// }

	// switch ev := c.Generic("runner").(*EnumValue).String(); ev {
	// case "local":
	// 	local := &runner.LocalRunner{}
	// 	local.Run(tp, seq)
	// case "docker":
	// 	fallthrough
	// default:
	// 	return fmt.Errorf("unsupported runner: %s", ev)
	// }

	return nil
}
