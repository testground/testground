package cmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mitchellh/mapstructure"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/client"
	"github.com/testground/testground/pkg/data"
	"github.com/testground/testground/pkg/logging"

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
	file := c.String("file")
	if file == "" {
		return fmt.Errorf("no composition file supplied")
	}

	comp, err := loadComposition(file)

	if err != nil {
		return fmt.Errorf("failed to load composition file: %w", err)
	}

	if err = comp.ValidateForBuild(); err != nil {
		return fmt.Errorf("invalid composition file: %w", err)
	}

	err = build(c, comp)
	if err != nil {
		return err
	}

	if c.Bool("write-artifacts") {
		err = api.WriteCompositionToFile(comp, file)
		if err != nil {
			return fmt.Errorf("failed to write composition file: %w", err)
		}
	}

	return nil
}


func buildSingleCmd(c *cli.Context) (err error) {
	var comp *api.Composition
	if comp, err = createSingletonComposition(c); err != nil {
		return err
	}
	err = build(c, comp)
	return err
}

func build(c *cli.Context, comp *api.Composition) error {
	cl, cfg, err := setupClient(c)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	log := logging.NewLogging(logging.NewLocalLogger(c.App.Writer, c.App.ErrWriter))

	// Resolve the test plan and its manifest.
	var manifest *api.TestPlanManifest
	planDir, manifest, err := resolveTestPlan(cfg, comp.Global.Plan)
	if err != nil {
		return fmt.Errorf("failed to resolve test plan: %w", err)
	}

	var (
		wait    = c.Bool("wait")
		sdkDir  string
	)

	log.S().Infof("test plan source at: %s", planDir)

	comp, err = comp.PrepareForBuild(manifest)
	if err != nil {
		return err
	}

	req := &api.BuildRequest{
		Composition: *comp,
		Manifest:    *manifest,
		CreatedBy: api.CreatedBy{
			User: cfg.Client.User,
		},
	}

	if wait {
		req.Priority = 1
	}

	// Resolve the linked SDK directory, if one has been supplied.
	if sdk := c.String("link-sdk"); sdk != "" {
		var err error
		sdkDir, err = resolveSDK(cfg, sdk)
		if err != nil {
			return fmt.Errorf("failed to resolve linked SDK directory: %w", err)
		}
		log.S().Infof("linking with sdk at: %s", sdkDir)
	}
	// if there are extra sources to include for this builder, contextualize
	// them to the plan's dir.
	builder := strings.Replace(comp.Global.Builder, ":", "_", -1)
	extra := manifest.ExtraSources[builder]
	log.S().Infof("build %s extra %s", builder, extra)
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

	id, err := client.ParseBuildResponse(resp, c.App.Writer)
	switch err {
	case nil:
	case context.Canceled:
		return fmt.Errorf("interrupted")
	default:
		return err
	}

	log.S().Infof("build queued with ID: %s", id)

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

	tsk, err := client.ParseLogsRequest(c.App.Writer, r)
	if err != nil {
		return err
	}

	if tsk.Error != "" {
		return errors.New(tsk.Error)
	}

	var artifactPaths []string
	err = mapstructure.Decode(tsk.Result, &artifactPaths)
	if err != nil {
		return err
	}

	for i, ap := range artifactPaths {
		g := comp.Groups[i]
		log.S().Infow("generated build artifact", "group", g.ID, "artifact", ap)
		g.Run.Artifact = ap
	}

	return data.IsTaskOutcomeInError(&tsk)
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

	err = client.ParseBuildPurgeResponse(resp, c.App.Writer)
	if err != nil {
		return err
	}

	fmt.Printf("finished purging testplan %s for builder %s\n", plan, builder)
	return nil
}

