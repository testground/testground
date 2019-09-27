package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ipfs/testground/pkg/runner"
	"github.com/ipfs/testground/pkg/util"

	"github.com/urfave/cli"
)

var runners = func() []string {
	r := _engine.ListRunners()
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
			},
			Usage: fmt.Sprintf("specifies the runner; options: %s", strings.Join(runners, ", ")),
		},
		cli.StringFlag{
			Name:  "nomad-api, n",
			Value: "http://127.0.0.1:5000",
			Usage: "the url of the Nomad endpoint (unused for now)",
		},
		cli.IntFlag{
			Name:  "instances, i",
			Usage: "number of instances of the test case to run",
		},
		cli.StringSliceFlag{
			Name:  "run-cfg",
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
		runcfg    = c.StringSlice("run-cfg")
		instances = c.Int("instances")
	)

	// Validate this test case was provided.
	if testcase == "" {
		cli.ShowSubcommandHelp(c)
		return errors.New("no test case provided; use the `list` command to view available test cases")
	}

	// Validate the test case format.
	comp := strings.Split(testcase, "/")
	if len(comp) != 2 {
		cli.ShowSubcommandHelp(c)
		return errors.New("wrong format for test case name, should be: `testplan/testcase`")
	}

	// Now that we've verified that the test plan and the test case exist, build
	// the testplan.
	buildIn, err := parseBuildInput(c)
	if err != nil {
		return fmt.Errorf("error while parsing build input: %w", err)
	}

	// Trigger the build job.
	buildOut, err := _engine.DoBuild(comp[0], builderId, buildIn)
	if err != nil {
		return fmt.Errorf("error while building test plan: %w", err)
	}

	// Process run cfg override.
	cfgOverride, err := util.ToOptionsMap(runcfg)
	if err != nil {
		return err
	}

	// Prepare the run job.
	runIn := &runner.Input{
		Instances:    instances,
		ArtifactPath: buildOut.ArtifactPath,
		RunnerConfig: cfgOverride,
	}

	_, err = _engine.DoRun(comp[0], comp[1], runnerId, runIn)
	return err
}
