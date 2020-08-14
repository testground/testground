package cmd

import (
	"context"
	"errors"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"github.com/testground/testground/pkg/logging"
	"os"
	"path/filepath"
	"time"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/client"

	"github.com/BurntSushi/toml"
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
				&cli.BoolFlag{
					Name:  "wait",
					Usage: "Wait for the run completion.",
				},
			),
		},
	},
}

func runCompositionCmd(c *cli.Context) (err error) {
	comp := new(api.Composition)
	file := c.String("file")
	if file == "" {
		return fmt.Errorf("no composition file supplied")
	}

	if _, err = toml.DecodeFile(file, comp); err != nil {
		return fmt.Errorf("failed to process composition file: %w", err)
	}

	if err = comp.ValidateForRun(); err != nil {
		return fmt.Errorf("invalid composition file: %w", err)
	}

	err = doRun(c, comp)
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
	return doRun(c, comp)
}

func doRun(c *cli.Context, comp *api.Composition) (err error) {
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

	// Check if this the daemon will need to build the project.
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
		builder := comp.Global.Builder
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

	req := &api.RunRequest{
		BuildGroups: buildIdx,
		Composition: *comp,
		Manifest:    *manifest,
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

	if !c.Bool("wait") {
		return nil
	}

	r, err := cl.TaskStatus(ctx, &api.TaskStatusRequest{
		ID:                id,
		WaitForCompletion: true,
	})
	if err != nil {
		return err
	}
	defer r.Close()

	res, err := client.ParseTaskStatusResponse(r)
	if err != nil {
		return err
	}

	time.Sleep(time.Second)

	// TODO: cancel task on context cancel

	/* select {
	case <-ctx.Done():
		fmt.Println("Should Cancel")
	}    */

	if res.Result.Error != "" {
		return errors.New(res.Result.Error)
	}

	var rout api.RunOutput
	err = mapstructure.Decode(res.Result.Data, &rout)
	if err != nil {
		return err
	}

	if file := c.String("file"); file != "" && c.Bool("write-artifacts") {
		f, err := os.OpenFile(file, os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to write composition to file: %w", err)
		}
		enc := toml.NewEncoder(f)
		if err := enc.Encode(rout.Composition); err != nil {
			return fmt.Errorf("failed to encode composition into file: %w", err)
		}
	}

	logging.S().Infof("finished run with ID: %s", id)

	// if the `collect` flag is not set, we are done, just return
	collectOpt := c.Bool("collect")
	if !collectOpt {
		return nil
	}

	collectFile := c.String("collect-file")
	if collectFile == "" {
		collectFile = fmt.Sprintf("%s.tgz", id)
	}

	return collect(ctx, cl, comp.Global.Runner, id, collectFile)
}
