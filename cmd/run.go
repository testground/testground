package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ipfs/testground/pkg/runner"

	"github.com/urfave/cli"
)

var runners = func() []string {
	r := Engine.ListRunners()
	if len(r) == 0 {
		panic("no runners loaded")
	}

	names := make([]string, 0, len(r))
	for k, _ := range r {
		names = append(names, k)
	}
	return names
}()

// RunCommand is the specification of the `run` command.
var RunCommand = cli.Command{
	Name:      "run",
	Usage:     "(builds and) runs test case with name `testplan/testcase`",
	Action:    runCommand,
	ArgsUsage: "[name]",
	Flags: append(
		BuildCommand.Flags, // inject all build command flags.
		cli.GenericFlag{
			Name: "runner, r",
			Value: &EnumValue{
				Allowed: runners,
				Default: runners[0],
			},
			Usage: fmt.Sprintf("specifies the runner; options: %v", runners),
		},
		cli.StringFlag{
			Name:  "nomad-api, n",
			Value: "http://127.0.0.1:5000",
			Usage: "the url of the Nomad endpoint (unused for now)",
		},
		cli.IntFlag{
			// default 0
			Name:  "instances, i",
			Usage: "number of instances of the test case to run",
		},
		cli.StringSliceFlag{
			Name:  "run-param",
			Usage: "provide a run parameter",
		},
	),
}

func runCommand(c *cli.Context) error {
	if c.NArg() != 1 {
		cli.ShowSubcommandHelp(c)
		return errors.New("missing test name")
	}

	// Extract flags and arguments.
	var (
		testcase  = c.Args().First()
		builderId = c.Generic("builder").(*EnumValue).String()
		runnerId  = c.Generic("runner").(*EnumValue).String()
		params    = c.StringSlice("run-param")
		instances = c.Int("instances")
	)

	// Validate this test plan and test case exist.
	if testcase == "" {
		cli.ShowSubcommandHelp(c)
		return errors.New("no test case provided; use the `list` command to view available test cases")
	}

	comp := strings.Split(testcase, "/")
	if len(comp) != 2 {
		cli.ShowSubcommandHelp(c)
		return errors.New("wrong format for test case name, should be: `testplan/testcase`")
	}

	tp := Engine.TestCensus().ByName(comp[0])
	if tp == nil {
		return errors.New("unrecognized test plan; use the `list` command to view available test plans and cases")
	}

	seq, _, ok := tp.TestCaseByName(comp[1])
	if !ok {
		return errors.New("unrecognized test case; use the `list` command to view available test cases")
	}

	// Slurp run parameters into a map.
	parameters, err := toKeyValues(params)
	if err != nil {
		return err
	}

	// Now that we've verified that the test plan and the test case exist, build
	// the testplan.
	buildIn, err := parseBuildInput(c)
	if err != nil {
		return fmt.Errorf("error while parsing build input: %w", err)
	}

	// Trigger the build job.
	buildOut, err := Engine.DoBuild(tp.Name, builderId, buildIn)
	if err != nil {
		return fmt.Errorf("error while building test plan: %w", err)
	}

	runIn := &runner.Input{
		TestPlan:      tp,
		Instances:     instances,
		Seq:           seq,
		ArtifactPath:  buildOut.ArtifactPath,
		RunParameters: parameters,
	}

	_, err = Engine.DoRun(tp.Name, runnerId, runIn)
	return err
}
