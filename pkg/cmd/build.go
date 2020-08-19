package cmd

import (
	"context"
	"errors"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"os"
	"path/filepath"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/client"
	"github.com/testground/testground/pkg/logging"

	"github.com/BurntSushi/toml"
	"github.com/urfave/cli/v2"
)

var linkSdkUsage = "links the test plan against a local SDK. The full `DIR_PATH`, or the NAME can be supplied, " +
	"in the latter case, the testground client will expect to find the SDK under $TESTGROUND_HOME/sdks/NAME"

var BuildCommand = cli.Command{
	Name:  "build",
	Usage: "request the daemon to build a test plan",
	Subcommands: cli.Commands{
		&cli.Command{
			Name:    "composition",
			Aliases: []string{"c"},
			Usage:   "builds a composition.",
			Action:  buildCompositionCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "file",
					Aliases:  []string{"f"},
					Usage:    "path to a `COMPOSITION`",
					Required: true,
				},
				&cli.BoolFlag{
					Name:    "write-artifacts",
					Aliases: []string{"w"},
					Usage:   "write the resulting build artifacts to the composition file",
				},
				&cli.StringFlag{
					Name:  "link-sdk",
					Usage: linkSdkUsage,
				},
				&cli.BoolFlag{
					Name:  "wait",
					Usage: "wait for the task to complete",
				},
			},
		},
		&cli.Command{
			Name:    "single",
			Aliases: []string{"s"},
			Usage:   "builds a single group, passing in all necessary input via CLI flags.",
			Action:  buildSingleCmd,
			Flags: cli.FlagsByName{
				&cli.StringSliceFlag{
					Name:  "build-cfg",
					Usage: "set a build config parameter",
				},
				&cli.StringFlag{
					Name:     "builder",
					Aliases:  []string{"b"},
					Usage:    "specifies the builder to use; values include: 'docker:go', 'exec:go'",
					Required: true,
				},
				&cli.StringSliceFlag{
					Name:    "dep",
					Aliases: []string{"d"},
					Usage:   "set a dependency mapping",
				},
				&cli.StringFlag{
					Name:  "link-sdk",
					Usage: linkSdkUsage,
				},
				&cli.StringFlag{
					Name:     "plan",
					Aliases:  []string{"p"},
					Usage:    "specifies the plan to run",
					Required: true,
				},
				&cli.BoolFlag{
					Name:  "wait",
					Usage: "Wait for the task to complete",
				},
			},
		},
		&cli.Command{
			Name:    "purge",
			Aliases: []string{"p"},
			Usage:   "purge the cache for a builder and testplan",
			Action:  runBuildPurgeCmd,
			Flags: cli.FlagsByName{
				&cli.StringFlag{
					Name:     "builder",
					Aliases:  []string{"b"},
					Usage:    "specifies the builder to use; values include: 'docker:go', 'exec:go'",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "plan",
					Aliases:  []string{"p"},
					Usage:    "specifies the plan to run",
					Required: true,
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

	err = doBuild(c, comp)
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
	err = doBuild(c, comp)
	return err
}

func doBuild(c *cli.Context, comp *api.Composition) error {
	var (
		plan    = comp.Global.Plan
		wait    = c.Bool("wait")
		planDir string
		sdkDir  string
	)

	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	cl, cfg, err := setupClient(c)
	if err != nil {
		return err
	}

	// Resolve the linked SDK directory, if one has been supplied.
	if sdk := c.String("link-sdk"); sdk != "" {
		var err error
		sdkDir, err = resolveSDK(cfg, sdk)
		if err != nil {
			return fmt.Errorf("failed to resolve linked SDK directory: %w", err)
		}
		logging.S().Infof("linking with sdk at: %s", sdkDir)
	}

	// Resolve the test plan and its manifest.
	var manifest *api.TestPlanManifest
	planDir, manifest, err = resolveTestPlan(cfg, plan)
	if err != nil {
		return fmt.Errorf("failed to resolve test plan: %w", err)
	}

	logging.S().Infof("test plan source at: %s", planDir)

	comp, err = comp.PrepareForBuild(manifest)
	if err != nil {
		return err
	}

	req := &api.BuildRequest{
		Composition: *comp,
		Manifest:    *manifest,
	}

	if wait {
		req.Priority = 1
	}

	// if there are extra sources to include for this builder, contextualize
	// them to the plan's dir.
	builder := comp.Global.Builder
	extra := manifest.ExtraSources[builder]
	for i, dir := range extra {
		if !filepath.IsAbs(dir) {
			// follow any symlinks in the plan dir.
			evalPlanDir, err := filepath.EvalSymlinks(planDir)
			if err != nil {
				return fmt.Errorf("failed to follow symlinks in plan dir: %w", err)
			}
			extra[i] = filepath.Clean(filepath.Join(evalPlanDir, dir))
		}
	}

	resp, err := cl.Build(ctx, req, planDir, sdkDir, extra)
	if err != nil {
		return err
	}
	defer resp.Close()

	id, err := client.ParseBuildResponse(resp)
	switch err {
	case nil:
	case context.Canceled:
		return fmt.Errorf("interrupted")
	default:
		return err
	}

	logging.S().Infof("build queued with ID: %s", id)

	if !wait {
		return nil
	}

	r, err := cl.Logs(ctx, &api.LogsRequest{
		TaskID:            id,
		Follow:            true,
		CancelWithContext: true,
	})
	if err != nil {
		return err
	}
	defer r.Close()

	tsk, err := client.ParseLogsRequest(r)
	if err != nil {
		return err
	}

	if tsk.Result.Error != "" {
		return errors.New(tsk.Result.Error)
	}

	var rout []api.BuildOutput
	err = mapstructure.Decode(tsk.Result.Data, &rout)
	if err != nil {
		return err
	}

	for i, out := range rout {
		g := comp.Groups[i]
		logging.S().Infow("generated build artifact", "group", g.ID, "artifact", out.ArtifactPath)
		g.Run.Artifact = out.ArtifactPath
	}

	return nil
}

func runBuildPurgeCmd(c *cli.Context) (err error) {
	var (
		builder = c.String("builder")
		plan    = c.String("plan")
	)

	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	cl, _, err := setupClient(c)
	if err != nil {
		return err
	}

	resp, err := cl.BuildPurge(ctx, &api.BuildPurgeRequest{
		Builder:  builder,
		Testplan: plan,
	})
	if err != nil {
		return err
	}
	defer resp.Close()

	err = client.ParseBuildPurgeResponse(resp)
	if err != nil {
		return err
	}

	fmt.Printf("finished purging testplan %s for builder %s\n", plan, builder)
	return nil
}
