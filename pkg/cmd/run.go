package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/client"
	"github.com/testground/testground/pkg/data"
	"github.com/testground/testground/pkg/logging"

	"github.com/urfave/cli/v2"
)

// RunCommand is the specification of the `run` command.
var RunCommand = cli.Command{
	Name:  "run",
	Usage: "request the daemon to (build and) run a test case",
	Subcommands: cli.Commands{
		&cli.Command{
			Name:    "composition",
			Aliases: []string{"c"},
			Usage:   "(build and) run a composition",
			Action:  runCompositionCmd,
			Flags: append(
				BuildCommand.Subcommands[0].Flags, // inject all build single command flags.
				&cli.BoolFlag{
					Name:    "ignore-artifacts",
					Aliases: []string{"i"},
					Usage:   "ignore any build artifacts present in the composition file",
				},
				&cli.BoolFlag{
					Name:  "collect",
					Usage: "collect assets at the end of the run phase; without --collect-file, it writes to <run_id>.tgz",
				},
				&cli.StringFlag{
					Name:    "collect-file",
					Aliases: []string{"o"},
					Usage:   "write the collection output archive to `FILENAME`",
				},
				&cli.StringFlag{
					Name:  "run-ids",
					Usage: "run a specific run id, or a comma-separated list of run ids",
				},
				&cli.StringFlag{
					Name:  "metadata-repo",
					Usage: "repo that triggered this run",
				},
				&cli.StringFlag{
					Name:  "metadata-branch",
					Usage: "branch that triggered this run",
				},
				&cli.StringFlag{
					Name:  "metadata-commit",
					Usage: "commit that triggered this run",
				},
			),
		},
		&cli.Command{
			Name:    "single",
			Aliases: []string{"s"},
			Usage:   "(build and) run a single group",
			Action:  runSingleCmd,
			Flags: append(
				BuildCommand.Subcommands[1].Flags, // inject all build single command flags.
				&cli.BoolFlag{
					Name:  "collect",
					Usage: "collect assets at the end of the run phase.",
				},
				&cli.StringFlag{
					Name:    "collect-file",
					Aliases: []string{"o"},
					Usage:   "destination for the assets if --collect is set",
				},
				&cli.UintFlag{
					Name:        "instances",
					Aliases:     []string{"i"},
					Usage:       "number of instances of the test case to run",
					Required:    true,
					DefaultText: "none",
				},
				&cli.StringFlag{
					Name:     "runner",
					Aliases:  []string{"r"},
					Usage:    "runner to use; values include: 'local:exec', 'local:docker', 'cluster:k8s'",
					Required: true,
				},
				&cli.StringSliceFlag{
					Name:  "run-cfg",
					Usage: "override runner configuration",
				},
				&cli.StringFlag{
					Name:     "testcase",
					Aliases:  []string{"t"},
					Usage:    "test case to run; must be defined in the test plan manifest",
					Required: true,
				},
				&cli.StringSliceFlag{
					Name:    "test-param",
					Aliases: []string{"tp"},
					Usage:   "set a test parameter",
				},
				&cli.StringFlag{
					Name:    "use-build",
					Aliases: []string{"ub"},
					Usage:   "build artifact to use (from a previous build)",
				},
				&cli.StringFlag{
					Name:  "metadata-repo",
					Usage: "repo that triggered this run",
				},
				&cli.StringFlag{
					Name:  "metadata-branch",
					Usage: "branch that triggered this run",
				},
				&cli.StringFlag{
					Name:  "metadata-commit",
					Usage: "commit that triggered this run",
				},
				&cli.BoolFlag{
					Name:  "disable-metrics",
					Usage: "disable metrics batching",
				},
			),
		},
	},
}

func runCompositionCmd(c *cli.Context) (err error) {
	file := c.String("file")
	if file == "" {
		return fmt.Errorf("no composition file supplied")
	}

	comp, err := loadComposition(file)

	if err != nil {
		return fmt.Errorf("failed to load composition file: %w", err)
	}

	if err = comp.ValidateForRun(); err != nil {
		return fmt.Errorf("invalid composition file: %w", err)
	}

	err = run(c, comp)
	if err != nil {
		return err
	}

	return nil
}

func runSingleCmd(c *cli.Context) (err error) {
	var comp *api.Composition
	if comp, err = createSingletonComposition(c); err != nil {
		return err
	}
	logging.S().Infof("created a synthetic composition file for this job; all instances will run under singleton group %q", comp.Groups[0].ID)
	return run(c, comp)
}

func run(c *cli.Context, comp *api.Composition) (err error) {
	// Assumes the composition has been validated for run.
	// TODO: Rethink the composition types to expose clearly what is validated / not validated.
	// TODO: In the composition we'll generate a default run if it is missing.
	cl, cfg, err := setupClient(c)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	// Resolve the test plan and its manifest.
	planDir, manifest, err := resolveTestPlan(cfg, comp.Global.Plan)
	if err != nil {
		return fmt.Errorf("failed to resolve test plan: %w", err)
	}

	// Retrieve the run ids to use.
	rawRunIds := c.String("run-ids")
	var runIds []string

	// default to all the runs in the composition
	if rawRunIds == "" {
		runIds = comp.ListRunIds()
	} else {
		runIds = strings.Split(rawRunIds, ",")
	}

	// TODO: validate run ids
	// TODO: verify run ids exists in the composition.

	// Skip artifacts if the user explicit requests it.
	// TODO: Simplify this code: empty the artifact field if required and post
	//       the composition to the daemon. The daemon will take care of identifying
	// 		 which artifacts should be built, etc.
	//		 Eventually we'll drop the BuildGroups field from the request.
	ignore := c.Bool("ignore-artifacts")
	var buildIdx []int
	for i, grp := range comp.Groups {
		if grp.Run.Artifact == "" || ignore {
			buildIdx = append(buildIdx, i)
		}
	}

	var (
		sdkDir    string
		extraSrcs []string
	)

	if len(buildIdx) > 0 {
		// Resolve the linked SDK directory, if one has been supplied.
		if sdk := c.String("link-sdk"); sdk != "" {
			var err error
			sdkDir, err = resolveSDK(cfg, sdk)
			if err != nil {
				return fmt.Errorf("failed to resolve linked SDK directory: %w", err)
			}
			logging.S().Infof("linking with sdk at: %s", sdkDir)
		}
		// if there are extra sources to include for this builder, contextualize
		// them to the plan's dir.
		builder := strings.Replace(comp.Global.Builder, ":", "_", -1)
		extraSrcs = manifest.ExtraSources[builder]
		for i, dir := range extraSrcs {
			if !filepath.IsAbs(dir) {
				// follow any symlinks in the plan dir.
				evalPlanDir, err := filepath.EvalSymlinks(planDir)
				if err != nil {
					return fmt.Errorf("failed to follow symlinks in plan dir: %w", err)
				}
				extraSrcs[i] = filepath.Clean(filepath.Join(evalPlanDir, dir))
			}
		}
	} else {
		planDir = ""
	}

	for _, runId := range runIds {
		// TODO: deal with preparing the composition to output the "general" version if possible.
		// Or do this in the supervisor.
		req := &api.RunRequest{
			BuildGroups: buildIdx,
			RunIds:      []string{runId},
			Composition: *comp,
			Manifest:    *manifest,
			CreatedBy: api.CreatedBy{
				User:   cfg.Client.User,
				Repo:   c.String("metadata-repo"),
				Branch: c.String("metadata-branch"),
				Commit: c.String("metadata-commit"),
			},
		}

		err := runSingle(ctx, cl, c, req, planDir, sdkDir, extraSrcs)

		if err != nil {
			return err
		}
	}

	return nil
}

func runSingle(ctx context.Context, cl *client.Client, c *cli.Context, req *api.RunRequest, planDir string, sdkDir string, extraSrcs []string) (err error) {
	var (
		collectOpt = c.Bool("collect")
		wait       = c.Bool("wait") || collectOpt // we always wait if we are collecting.
	)

	if wait {
		req.Priority = 1
	}

	resp, err := cl.Run(ctx, req, planDir, sdkDir, extraSrcs)
	switch err {
	case nil:
		// noop
	case context.Canceled:
		return fmt.Errorf("interrupted")
	default:
		return err
	}

	defer resp.Close()

	id, err := client.ParseRunResponse(resp)
	if err != nil {
		return err
	}

	logging.S().Infof("run is queued with ID: %s", id)

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

	tsk, err := client.ParseLogsRequest(os.Stdout, r)
	if err != nil {
		return err
	}

	if tsk.Error != "" {
		return errors.New(tsk.Error)
	}

	// TODO: switch this depending on whether this is a multi-runs or a simple run.
	var composition api.Composition
	err = mapstructure.Decode(tsk.Composition, &composition)
	if err != nil {
		return err
	}

	if file := c.String("file"); file != "" && c.Bool("write-artifacts") {
		err = api.WriteCompositionToFile(&composition, file)
		if err != nil {
			return fmt.Errorf("failed to write composition file: %w", err)
		}
	}

	logging.S().Infof("finished run with ID: %s", id)

	// if the `collect` flag is not set, we are done
	if !collectOpt {
		return data.IsTaskOutcomeInError(&tsk)
	}

	// TODO: switch this depending on whether this is a multi-runs or a simple run.
	collectFile := c.String("collect-file")
	if collectFile == "" {
		collectFile = fmt.Sprintf("%s.tgz", id)
	}

	err = collect(ctx, cl, req.Composition.Global.Runner, id, collectFile)

	if err != nil {
		return cli.Exit(err.Error(), 3)
	}

	return data.IsTaskOutcomeInError(&tsk)
}
