package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/testground/testground/pkg/logging"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/client"

	"github.com/BurntSushi/toml"
	"github.com/urfave/cli/v2"
)

// RunCommand is the specification of the `run` command.
var RunCommand = cli.Command{
	Name:  "run",
	Usage: "(Builds and) runs a test case. List test cases with the `list` command.",
	Subcommands: cli.Commands{
		&cli.Command{
			Name:    "composition",
			Aliases: []string{"c"},
			Usage:   "(Builds and) runs a composition.",
			Action:  runCompositionCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "file",
					Aliases: []string{"f"},
					Usage:   "path to a composition `FILE`",
				},
				&cli.BoolFlag{
					Name:    "write-artifacts",
					Aliases: []string{"w"},
					Usage:   "writes the resulting build artifacts to the composition file.",
				},
				&cli.BoolFlag{
					Name:    "ignore-artifacts",
					Aliases: []string{"i"},
					Usage:   "ignores any build artifacts present in the composition file.",
				},
				&cli.BoolFlag{
					Name:  "collect",
					Usage: "collect assets at the end of the run phase.",
				},
				&cli.StringFlag{
					Name:    "collect-file",
					Aliases: []string{"o"},
					Usage:   "destination for the assets if --collect is set",
				},
			},
		},
		&cli.Command{
			Name:    "single",
			Aliases: []string{"s"},
			Usage:   "(Builds and) runs a single group.",
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
					Name:     "instances",
					Aliases:  []string{"i"},
					Usage:    "number of instances of the test case to run",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "runner",
					Aliases:  []string{"r"},
					Usage:    "specifies the runner to use; values include: 'local:exec', 'local:docker', 'cluster:k8s'",
					Required: true,
				},
				&cli.StringSliceFlag{
					Name:  "run-cfg",
					Usage: "override runner configuration",
				},
				&cli.StringFlag{
					Name:     "testcase",
					Aliases:  []string{"t"},
					Usage:    "specifies the test case. must be defined by the test plan manifest.",
					Required: true,
				},
				&cli.StringSliceFlag{
					Name:    "test-param",
					Aliases: []string{"tp"},
					Usage:   "provide a test parameter",
				},
				&cli.StringFlag{
					Name:    "use-build",
					Aliases: []string{"ub"},
					Usage:   "specifies the artifact to use (from a previous build)",
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
	var manifest *api.TestPlanManifest
	_, manifest, err = resolveTestPlan(cfg, comp.Global.Plan)
	if err != nil {
		return fmt.Errorf("failed to resolve test plan: %w", err)
	}

	// Check if we have any groups without an build artifact; if so, trigger a
	// build for those.
	var buildIdx []int
	ignore := c.Bool("ignore-artifacts")
	for i, grp := range comp.Groups {
		if grp.Run.Artifact == "" || ignore {
			buildIdx = append(buildIdx, i)
		}
	}

	if len(buildIdx) > 0 {
		bcomp, err := comp.PickGroups(buildIdx...)
		if err != nil {
			return err
		}

		bout, err := doBuild(c, &bcomp)
		if err != nil {
			return err
		}

		// Populate the returned build IDs.
		for i, groupIdx := range buildIdx {
			g := &comp.Groups[groupIdx]
			g.Run.Artifact = bout[i].ArtifactPath
		}

		if file := c.String("file"); file != "" && c.Bool("write-artifacts") {
			f, err := os.OpenFile(file, os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("failed to write composition to file: %w", err)
			}
			enc := toml.NewEncoder(f)
			if err := enc.Encode(comp); err != nil {
				return fmt.Errorf("failed to encode composition into file: %w", err)
			}
		}
	}

	comp, err = comp.PrepareForRun(manifest)
	if err != nil {
		return err
	}

	req := &api.RunRequest{
		Composition: *comp,
	}

	resp, err := cl.Run(ctx, req)
	switch err {
	case nil:
		// noop
	case context.Canceled:
		return fmt.Errorf("interrupted")
	default:
		return err
	}

	defer resp.Close()

	rout, err := client.ParseRunResponse(resp)
	if err != nil {
		return err
	}

	logging.S().Infof("finished run with ID: %s", rout.RunID)

	// if the `collect` flag is not set, we are done, just return
	collectOpt := c.Bool("collect")
	if !collectOpt {
		return nil
	}

	collectFile := c.String("collect-file")
	if collectFile == "" {
		collectFile = fmt.Sprintf("%s.tgz", rout.RunID)
	}

	return collect(ctx, cl, comp.Global.Runner, rout.RunID, collectFile)
}
