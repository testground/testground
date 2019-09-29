package cmd

import (
	"errors"

	"github.com/ipfs/testground/pkg/build"
	"github.com/ipfs/testground/pkg/util"

	"github.com/davecgh/go-spew/spew"
	"github.com/urfave/cli"
)

var builders = func() []string {
	b := _engine.ListBuilders()
	if len(b) == 0 {
		panic("no builders loaded")
	}

	names := make([]string, 0, len(b))
	for k, _ := range b {
		names = append(names, k)
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

	in, err := parseBuildInput(c)
	if err != nil {
		return err
	}

	out, err := _engine.DoBuild(plan, builder, in)
	if err != nil {
		return err
	}

	spew.Dump(out)

	return nil
}

func parseBuildInput(c *cli.Context) (*build.Input, error) {
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

	in := &build.Input{
		Dependencies: dependencies,
		BuildConfig:  config,
	}
	return in, err
}
