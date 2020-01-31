package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/client"
	"github.com/ipfs/testground/pkg/engine"
	"github.com/ipfs/testground/pkg/logging"

	"github.com/BurntSushi/toml"
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
	Name:  "build",
	Usage: "builds a test plan",
	Subcommands: cli.Commands{
		cli.Command{
			Name:    "composition",
			Aliases: []string{"c"},
			Usage:   "Builds a composition.",
			Action:  buildCompositionCmd,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "file, f",
					Usage: "path to a composition `FILE`",
				},
				cli.BoolFlag{
					Name:  "write-artifacts, w",
					Usage: "Writes the resulting build artifacts to the composition file.",
				},
			},
		},
		cli.Command{
			Name:      "single",
			Aliases:   []string{"s"},
			Usage:     "Builds a single group, passing in all necesssary input via CLI flags.",
			Action:    buildSingleCmd,
			ArgsUsage: "[<testplan>]",
			Flags: []cli.Flag{
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
		},
	},
}

func buildCompositionCmd(c *cli.Context) (err error) {
	comp := new(api.Composition)
	file := c.String("file")
	if file == "" {
		return fmt.Errorf("no composition file supplied")
	}

	if _, err = toml.DecodeFile(file, comp); err != nil {
		return fmt.Errorf("failed to process composition file: %w", err)
	}
	if err = comp.Validate(); err != nil {
		return fmt.Errorf("invalid composition file: %w", err)
	}

	_, err = doBuild(c, comp)
	if err != nil {
		return err
	}

	if c.Bool("write-artifacts") {
		f, err := os.OpenFile(file, os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to write composition to file: %w", err)
		}
		enc := toml.NewEncoder(f)
		if err := enc.Encode(comp); err != nil {
			return fmt.Errorf("failed to encode composition into file: %w", err)
		}
	}

	return nil
}

func buildSingleCmd(c *cli.Context) (err error) {
	var comp *api.Composition
	if comp, err = createSingletonComposition(c); err != nil {
		return err
	}
	_, err = doBuild(c, comp)
	return err
}

func doBuild(c *cli.Context, comp *api.Composition) ([]api.BuildOutput, error) {
	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	cl, err := setupClient(c)
	if err != nil {
		return nil, err
	}

	req := &client.BuildRequest{Composition: *comp}
	resp, err := cl.Build(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("fatal error from daemon: %s", err)
	}
	defer resp.Close()

	res, err := client.ParseBuildResponse(resp)
	switch err {
	case nil:
	case context.Canceled:
		return nil, fmt.Errorf("interrupted")
	default:
		return nil, fmt.Errorf("fatal error from daemon: %w", err)
	}

	for i, out := range res {
		g := &comp.Groups[i]
		logging.S().Infow("generated build artifact", "group", g.ID, "artifact", out.ArtifactPath)
		g.Run.Artifact = out.ArtifactPath
	}
	return res, nil
}
