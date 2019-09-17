package cmd

import (
	"errors"

	"github.com/ipfs/testground/pkg/build"

	"github.com/davecgh/go-spew/spew"
	"github.com/urfave/cli"
)

var builders = func() []string {
	b := Engine.ListBuilders()
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
				Default: builders[0],
			},
		},
		cli.StringSliceFlag{
			Name:  "dep, d",
			Usage: "set a dependency mapping",
		},
		cli.StringSliceFlag{
			Name:  "build-param",
			Usage: "set a build parameter",
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

	out, err := Engine.DoBuild(plan, builder, in)
	if err != nil {
		return err
	}

	spew.Dump(out)

	return nil
}

func parseBuildInput(c *cli.Context) (*build.Input, error) {
	var (
		deps   = c.StringSlice("dep")
		params = c.StringSlice("build-param")
	)

	dependencies, err := toKeyValues(deps)
	if err != nil {
		return nil, err
	}

	parameters, err := toKeyValues(params)
	if err != nil {
		return nil, err
	}

	in := &build.Input{
		Dependencies:    dependencies,
		BuildParameters: parameters,
	}
	return in, err
}
