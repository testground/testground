package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/client"
	"github.com/ipfs/testground/pkg/engine"
	"github.com/ipfs/testground/pkg/logging"

	"github.com/BurntSushi/toml"
	"github.com/urfave/cli"
)

var runners = func() []string {
	names := make([]string, 0, len(engine.AllRunners))
	for _, r := range engine.AllRunners {
		names = append(names, r.ID())
	}
	return names
}()

// RunCommand is the specification of the `run` command.
var RunCommand = cli.Command{
	Name:      "run",
	Usage:     "(builds and) runs test case with name `<testplan>/<testcase>`. List test cases with `list` command",
	Action:    runCommand,
	ArgsUsage: "[name]",
	Flags: append(
		BuildCommand.Flags, // inject all build command flags.
		cli.GenericFlag{
			Name: "runner, r",
			Value: &EnumValue{
				Allowed: runners,
				Default: "local:exec",
			},
			Usage: fmt.Sprintf("specifies the runner; options: %s", strings.Join(runners, ", ")),
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
}

func runCommand(c *cli.Context) (err error) {
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

	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	// Check if we have any groups without an build artifact; if so, trigger a
	// build for those.
	var retain []int
	for i, grp := range comp.Groups {
		if grp.Run.Artifact == "" {
			retain = append(retain, i)
		}
	}

	if len(retain) > 0 {
		bcomp, err := comp.PickGroups(retain...)
		if err != nil {
			return err
		}

		resp, err := cl.Build(ctx, &client.BuildRequest{Composition: bcomp})
		switch err {
		case nil:
			// noop
		case context.Canceled:
			return fmt.Errorf("interrupted")
		default:
			return fmt.Errorf("fatal error from daemon: %w", err)
		}

		defer resp.Close() // yeah, Close could be called sooner, but it's not worth the verbosity.

		bout, err := client.ParseBuildResponse(resp)
		if err != nil {
			return err
		}

		// Populate the returned build IDs.
		for i, groupIdx := range retain {
			logging.S().Infow("generated build artifact", "group", comp.Groups[groupIdx].ID, "artifact", bout[i].ArtifactPath)
			comp.Groups[groupIdx].Run.Artifact = bout[i].ArtifactPath
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

	return client.ParseRunResponse(resp)
}
