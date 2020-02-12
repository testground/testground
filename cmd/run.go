package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/ipfs/testground/pkg/logging"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/client"

	"github.com/BurntSushi/toml"
	"github.com/urfave/cli"
)

// RunCommand is the specification of the `run` command.
var RunCommand = cli.Command{
	Name:  "run",
	Usage: "(Builds and) runs a test case. List test cases with the `list` command.",
	Subcommands: cli.Commands{
		cli.Command{
			Name:    "composition",
			Aliases: []string{"c"},
			Usage:   "(Builds and) runs a composition.",
			Action:  runCompositionCmd,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "file, f",
					Usage: "path to a composition `FILE`",
				},
				cli.BoolFlag{
					Name:  "write-artifacts, w",
					Usage: "Writes the resulting build artifacts to the composition file.",
				},
				cli.BoolFlag{
					Name:  "ignore-artifacts, i",
					Usage: "Ignores any build artifacts present in the composition file.",
				},
				cli.BoolFlag{
					Name:  "collect",
					Usage: "Collect assets at the end of the run phase.",
				},
				cli.StringFlag{
					Name:  "collect-file, o",
					Usage: "Destination for the assets if --collect is set",
				},
			},
		},
		cli.Command{
			Name:      "single",
			Aliases:   []string{"s"},
			Usage:     "(Builds and) runs a single group.",
			Action:    runSingleCmd,
			ArgsUsage: "[name]",
			Flags: append(
				BuildCommand.Subcommands[1].Flags, // inject all build single command flags.
				cli.StringFlag{
					Name:  "runner, r",
					Usage: "specifies the runner to use; values include: 'local:exec', 'local:docker', 'cluster:k8s'",
				},
				cli.StringFlag{
					Name:  "use-build, ub",
					Usage: "specifies the artifact to use (from a previous build)",
				},
				cli.UintFlag{
					Name:  "instances, i",
					Usage: "number of instances of the test case to run",
				},
				cli.StringSliceFlag{
					Name:  "run-cfg",
					Usage: "override runner configuration",
				},
				cli.StringSliceFlag{
					Name:  "test-param, p",
					Usage: "provide a test parameter",
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
	cl, err := setupClient(c)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

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

	req := &client.RunRequest{
		Composition: *comp,
	}

	resp, err := cl.Run(ctx, req)
	switch err {
	case nil:
		// noop
	case context.Canceled:
		return fmt.Errorf("interrupted")
	default:
		return fmt.Errorf("fatal error from daemon: %w", err)
	}

	defer resp.Close()

	rout, err := client.ParseRunResponse(resp)
	if err != nil {
		return err
	}

	logging.S().Infof("finished run with ID: %s", rout.RunID)

	// if the `collect` flag is not set, we are done, just return
	collect := c.Bool("collect")
	if !collect {
		return nil
	}

	collectFile := c.String("collect-file")
	if collectFile == "" {
		collectFile = fmt.Sprintf("%s.zip", rout.RunID)
	}

	or := &client.OutputsRequest{
		Runner: comp.Global.Runner,
		RunID:  rout.RunID,
	}

	rc, err := cl.CollectOutputs(ctx, or)

	file, err := os.Create(collectFile)
	if err != nil {
		if err == context.Canceled {
			return fmt.Errorf("interrupted")
		}
		return fmt.Errorf("fatal error from daemon: %s", err)
	}
	defer file.Close()

	_, err = io.Copy(file, rc)
	if err != nil {
		return err
	}

	logging.S().Infof("created file: %s", collectFile)
	return nil
}
