package cmd

import (
	"context"
	"fmt"
	"github.com/ipfs/testground/pkg/logging"

	"github.com/BurntSushi/toml"
	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/client"
	"github.com/ipfs/testground/pkg/engine"

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
		cli.StringFlag{
			Name:  "file, f",
			Usage: "path to a composition `FILE`; this flag is exclusive of all other flags",
		},
		cli.GenericFlag{
			Name: "builder, b",
			Value: &EnumValue{
				Allowed: builders,
				Default: "exec:go",
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

func buildCommand(c *cli.Context) (err error) {
	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	comp := new(api.Composition)
	if file := c.String("file"); file != "" {
		if c.NumFlags() > 2 {
			// NumFlags counts 1 flag per variant, i.e. f and file are
			// considered towards the count, despite one being an alias for the
			// other.
			return fmt.Errorf("composition files are incompatible with all other CLI flags")
		}

		if _, err = toml.DecodeFile(file, comp); err != nil {
			return fmt.Errorf("failed to process composition file: %w", err)
		}

		if err = comp.Validate(); err != nil {
			return fmt.Errorf("invalid composition file: %w", err)
		}
	} else {
		if comp, err = createSingletonComposition(c); err != nil {
			return err
		}
	}

	cl, err := setupClient(c)
	if err != nil {
		return err
	}

	req := &client.BuildRequest{Composition: *comp}
	resp, err := cl.Build(ctx, req)
	if err != nil {
		return fmt.Errorf("fatal error from daemon: %s", err)
	}
	defer resp.Close()

	switch res, err := client.ParseBuildResponse(resp); err {
	case nil:
		for i, out := range res {
			logging.S().Infow("generated build artifact", "group", comp.Groups[i].ID, "artifact", out.ArtifactPath)
		}
		return nil
	case context.Canceled:
		return fmt.Errorf("interrupted")
	default:
		return fmt.Errorf("fatal error from daemon: %w", err)
	}
}
