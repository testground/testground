package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/client"
	"github.com/ipfs/testground/pkg/config"
	"github.com/ipfs/testground/pkg/conv"
	"github.com/urfave/cli"
)

// Commands collects all subcommands of the testground CLI.
var Commands = []cli.Command{
	RunCommand,
	ListCommand,
	BuildCommand,
	DescribeCommand,
	SidecarCommand,
	DaemonCommand,
	CollectCommand,
	TerminateCommand,
	HealthcheckCommand,
}

var Flags = []cli.Flag{
	cli.BoolFlag{
		Name:  "v",
		Usage: "verbose output (equivalent to DEBUG log level)",
	},
	cli.BoolFlag{
		Name:  "vv",
		Usage: "super verbose output (equivalent to DEBUG log level for now, it may accommodate TRACE in the future)",
	},
	cli.StringFlag{
		Name:  "endpoint",
		Usage: "set the daemon endpoint URI (overrides .env.toml)",
	},
}

func setupClient(c *cli.Context) (*client.Client, error) {
	endpoint := c.GlobalString("endpoint")

	if endpoint == "" {
		envcfg, err := config.GetEnvConfig()
		if err != nil {
			return nil, err
		}
		endpoint = envcfg.Client.Endpoint
	}

	api := client.New(endpoint)
	return api, nil
}

func createSingletonComposition(c *cli.Context) (*api.Composition, error) {
	var (
		testcase = c.Args().First()

		builder   = c.String("builder")
		runner    = c.String("runner")
		instances = c.Uint("instances")
		artifact  = c.String("use-build")

		buildcfg     = c.StringSlice("build-cfg")
		dependencies = c.StringSlice("dep")

		runcfg     = c.StringSlice("run-cfg")
		testparams = c.StringSlice("test-param")
	)

	comp := &api.Composition{
		Global: api.Global{
			Builder:        builder,
			Runner:         runner,
			TotalInstances: instances,
		},
		Groups: []api.Group{
			api.Group{
				ID: "single",
				Instances: api.Instances{
					Count: instances,
				},
				Run: api.Run{
					Artifact: artifact,
				},
			},
		},
	}

	// Validate the test case format.
	switch ss := strings.Split(testcase, "/"); len(ss) {
	case 0:
		_ = cli.ShowSubcommandHelp(c)
		return nil, errors.New("wrong format for test case name, should be: `testplan/testcase`")
	case 2:
		comp.Global.Case = ss[1]
		fallthrough
	case 1:
		comp.Global.Plan = ss[0]
	default:
		return nil, errors.New("wrong format for test case name, should be: `testplan/testcase`")
	}

	// Build configuration.
	config, err := conv.ParseKeyValues(buildcfg)
	if err != nil {
		return nil, fmt.Errorf("failed while parsing build config: %w", err)
	}
	comp.Global.BuildConfig = conv.InferTypedMap(config)

	// Run configuration.
	config, err = conv.ParseKeyValues(runcfg)
	if err != nil {
		return nil, fmt.Errorf("failed while parsing run config: %w", err)
	}
	comp.Global.RunConfig = conv.InferTypedMap(config)

	// Test parameters.
	parameters, err := conv.ParseKeyValues(testparams)
	if err != nil {
		return nil, fmt.Errorf("failed while parsing test paremters: %w", err)
	}
	comp.Groups[0].Run.TestParams = parameters

	deps, err := conv.ParseKeyValues(dependencies)
	if err != nil {
		return nil, err
	}
	comp.Groups[0].Build.Dependencies = make([]api.Dependency, 0, len(dependencies))

	for name, ver := range deps {
		dep := api.Dependency{
			Module:  name,
			Version: ver,
		}
		comp.Groups[0].Build.Dependencies = append(comp.Groups[0].Build.Dependencies, dep)
	}

	switch c := strings.Fields(c.Command.FullName()); c[0] {
	case "build":
		err = comp.ValidateForBuild()
	case "run":
		err = comp.ValidateForRun()
	default:
		err = errors.New("unexpected command")
	}

	return comp, err
}
