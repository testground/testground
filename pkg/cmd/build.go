package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/client"
	"github.com/ipfs/testground/pkg/logging"

	"github.com/BurntSushi/toml"
	"github.com/urfave/cli/v2"
)

var BuildCommand = cli.Command{
	Name:  "build",
	Usage: "builds a test plan",
	Subcommands: cli.Commands{
		&cli.Command{
			Name:    "composition",
			Aliases: []string{"c"},
			Usage:   "Builds a composition.",
			Action:  buildCompositionCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "file, f",
					Usage: "path to a composition `FILE`",
				},
				&cli.BoolFlag{
					Name:  "write-artifacts, w",
					Usage: "Writes the resulting build artifacts to the composition file.",
				},
				&cli.StringFlag{
					Name: "link-sdk",
					Usage: "links the test plan against a local SDK. The full `DIR_PATH`, or the `NAME` can be supplied," +
						"In the latter case, the testground client will expect to find the SDK under $TESTGROUND_HOME/sdks/NAME",
				},
			},
		},
		&cli.Command{
			Name:      "single",
			Aliases:   []string{"s"},
			Usage:     "Builds a single group, passing in all necesssary input via CLI flags.",
			Action:    buildSingleCmd,
			ArgsUsage: "[<testplan>]",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "builder, b",
					Usage: "specifies the builder to use; values include: 'docker:go', 'exec:go'",
				},
				&cli.StringSliceFlag{
					Name:  "dep, d",
					Usage: "set a dependency mapping",
				},
				&cli.StringSliceFlag{
					Name:  "build-cfg",
					Usage: "set a build config parameter",
				},
				&cli.StringFlag{
					Name: "link-sdk",
					Usage: "links the test plan against a local SDK. The full `DIR_PATH`, or the `NAME` can be supplied," +
						"In the latter case, the testground client will expect to find the SDK under $TESTGROUND_HOME/sdks/NAME",
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
	if err = comp.ValidateForBuild(); err != nil {
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
	var (
		plan    = comp.Global.Plan
		planDir string
		sdkDir  string
	)

	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	cl, cfg, err := setupClient(c)
	if err != nil {
		return nil, err
	}

	// Resolve the linked SDK directory, if one has been supplied.
	if sdk := c.String("link-sdk"); sdk != "" {
		var err error
		sdkDir, err = resolveSDK(cfg, sdk)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve linked SDK directory: %w", err)
		}
		logging.S().Infof("linking with sdk at: %s", sdkDir)
	}

	// Resolve the test plan and its manifest.
	var manifest *api.TestPlanManifest
	planDir, manifest, err = resolveTestPlan(cfg, plan)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve test plan: %w", err)
	}

	logging.S().Infof("test plan source at: %s", planDir)

	comp, err = comp.PrepareForBuild(manifest)
	if err != nil {
		return nil, err
	}

	req := &api.BuildRequest{Composition: *comp}
	resp, err := cl.Build(ctx, req, planDir, sdkDir)
	if err != nil {
		return nil, err
	}
	defer resp.Close()

	res, err := client.ParseBuildResponse(resp)
	switch err {
	case nil:
	case context.Canceled:
		return nil, fmt.Errorf("interrupted")
	default:
		return nil, err
	}

	for i, out := range res {
		g := &comp.Groups[i]
		logging.S().Infow("generated build artifact", "group", g.ID, "artifact", out.ArtifactPath)
		g.Run.Artifact = out.ArtifactPath
	}
	return res, nil
}
