package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/daemon/client"
	"github.com/ipfs/testground/pkg/engine"
	"github.com/ipfs/testground/pkg/util"

	"github.com/urfave/cli"
)

var builders = func() []string {
	names := make([]string, 0, len(engine.AllBuilders))
	for _, b := range engine.AllBuilders {
		names = append(names, b.ID())
	}
	return names
}()

var BuildCommand = cli.Command{
	Name:      "build",
	Usage:     "builds a test plan",
	Action:    buildCommand,
	ArgsUsage: "[<testplan>]",
	Flags: []cli.Flag{
		cli.GenericFlag{
			Name: "builder, b",
			Value: &EnumValue{
				Allowed: builders,
			},
		},
		cli.StringSliceFlag{
			Name:  "dep, d",
			Usage: "set a dependency mapping",
		},
		cli.StringSliceFlag{
			Name:  "build-cfg",
			Usage: "set a build config parameter",
		},
	},
	Description: `Builds a test plan by name. It errors if the test plan doesn't exist. Otherwise, it runs the build and outputs the Docker image id.

	 This command is prepared to produce different types of outputs, but only the docket output is supported at this time.
	`,
}

func buildCommand(c *cli.Context) error {
	if c.NArg() != 1 {
		return errors.New("test plan name must be provided")
	}

	var (
		plan    = c.Args().First()
		builder = c.Generic("builder").(*EnumValue).String()
	)

	api, cancel, err := setupClient()
	if err != nil {
		return err
	}
	defer cancel()

	in, err := parseBuildInput(c)
	if err != nil {
		return err
	}

	req := &client.BuildRequest{
		Dependencies: in.Dependencies,
		BuildConfig:  in.BuildConfig,
		Plan:         plan,
		Builder:      builder,
	}

	resp, err := api.Build(context.Background(), req)
	if err != nil {
		return fmt.Errorf("fatal error from daemon: %s", err)
	}
	defer resp.Close()

	_, err = client.ParseBuildResponse(resp)
	return err
}

func parseBuildInput(c *cli.Context) (*api.BuildInput, error) {
	var (
		deps = c.StringSlice("dep")
		cfg  = c.StringSlice("build-cfg")
	)

	d, err := util.ToOptionsMap(deps, false)
	if err != nil {
		return nil, err
	}
	dependencies, err := util.ToStringStringMap(d)
	if err != nil {
		return nil, err
	}

	config, err := util.ToOptionsMap(cfg, true)
	if err != nil {
		return nil, err
	}

	in := &api.BuildInput{
		Dependencies: dependencies,
		BuildConfig:  config,
	}
	return in, err
}
