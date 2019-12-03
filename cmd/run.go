package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ipfs/testground/pkg/daemon/client"
	"github.com/ipfs/testground/pkg/engine"
	"github.com/ipfs/testground/pkg/util"

	"github.com/urfave/cli"
)

var runners = func() []string {
	names := make([]string, 0, len(engine.AllRunners))
	for _, r := range engine.AllRunners {
		names = append(names, r.ID())
	}
	return names
}()

// RunCommand is the specification of the `run` command.
var RunCommand = cli.Command{
	Name:      "run",
	Usage:     "(builds and) runs test case with name `<testplan>/<testcase>`. List test cases with `list` command",
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
			Name:  "use-build, ub",
			Usage: "specifies the artifact to use (from a previous build)",
		},
		cli.IntFlag{
			Name:  "instances, i",
			Usage: "number of instances of the test case to run",
		},
		cli.StringSliceFlag{
			Name:  "run-cfg",
			Usage: "override runner configuration",
		},
		cli.StringSliceFlag{
			Name:  "test-param, p",
			Usage: "provide a test parameter",
		},
	),
}

func runCommand(c *cli.Context) error {
	if c.NArg() != 1 {
		_ = cli.ShowSubcommandHelp(c)
		return errors.New("missing test name")
	}

	// Extract flags and arguments.
	var (
		testcase     = c.Args().First()
		builderId    = c.Generic("builder").(*EnumValue).String()
		runnerId     = c.Generic("runner").(*EnumValue).String()
		runcfg       = c.StringSlice("run-cfg")
		instances    = c.Int("instances")
		testparams   = c.StringSlice("test-param")
		artifactPath = c.String("use-build")
	)

	// Validate this test case was provided.
	if testcase == "" {
		_ = cli.ShowSubcommandHelp(c)
		return errors.New("no test case provided; use the `list` command to view available test cases")
	}

	// Validate the test case format.
	comp := strings.Split(testcase, "/")
	if len(comp) != 2 {
		_ = cli.ShowSubcommandHelp(c)
		return errors.New("wrong format for test case name, should be: `testplan/testcase`")
	}

	api, cancel, err := setupClient()
	if err != nil {
		return err
	}
	defer cancel()

	if artifactPath == "" {
		// Now that we've verified that the test plan and the test case exist, build
		// the testplan.
		in, err := parseBuildInput(c)
		if err != nil {
			return fmt.Errorf("error while parsing build input: %w", err)
		}

		req := &client.BuildRequest{
			Dependencies: in.Dependencies,
			BuildConfig:  in.BuildConfig,
			Plan:         comp[0],
			Builder:      builderId,
		}

		resp, err := api.Build(context.Background(), req)
		if err != nil {
			return err
		}

		artifactPath, err = client.ProcessBuildResponse(resp)
		if err != nil {
			return err
		}
	}

	// Process run cfg override.
	cfgOverride, err := util.ToOptionsMap(runcfg, true)
	if err != nil {
		return err
	}

	// Pick up test parameters.
	p, err := util.ToOptionsMap(testparams, false)
	if err != nil {
		return err
	}
	parameters, err := util.ToStringStringMap(p)
	if err != nil {
		return err
	}

	runReq := &client.RunRequest{
		Plan:         comp[0],
		Case:         comp[1],
		Runner:       runnerId,
		Instances:    instances,
		ArtifactPath: artifactPath,
		Parameters:   parameters,
		RunnerConfig: cfgOverride,
	}

	resp, err := api.Run(context.Background(), runReq)
	if err != nil {
		return err
	}
	defer resp.Close()

	scanner := bufio.NewScanner(resp)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	return nil
}
