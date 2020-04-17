package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/config"

	"github.com/BurntSushi/toml"
	"github.com/mattn/go-zglob"
	"github.com/urfave/cli/v2"
)

// ListCommand is the specification of the `list` command.
var ListCommand = cli.Command{
	Name:      "list",
	Usage:     "enumerate all test cases known to the client",
	ArgsUsage: " ",
	Action:    listCommand,
}

func listCommand(c *cli.Context) error {
	cfg := &config.EnvConfig{}
	if err := cfg.Load(); err != nil {
		return err
	}

	manifests, err := zglob.GlobFollowSymlinks(filepath.Join(cfg.Dirs().Plans(), "**", "manifest.toml"))
	if err != nil {
		return fmt.Errorf("failed to discover test plans under %s: %w", cfg.Dirs().Plans(), err)
	}

	for _, file := range manifests {
		dir := filepath.Dir(file)

		plan, err := filepath.Rel(cfg.Dirs().Plans(), dir)
		if err != nil {
			return fmt.Errorf("failed to relativize plan directory %s: %w", dir, err)
		}

		var manifest api.TestPlanManifest
		if _, err = toml.DecodeFile(file, &manifest); err != nil {
			return fmt.Errorf("failed to process manifest file at %s: %w", file, err)
		}

		for _, tc := range manifest.TestCases {
			fmt.Println(plan + ":" + tc.Name)
		}
	}

	return nil
}
