package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/config"

	"github.com/BurntSushi/toml"
	"github.com/go-git/go-git/v5"
	"github.com/mattn/go-zglob"
	"github.com/urfave/cli/v2"
)

var PlanCommand = cli.Command{
	Name:  "plan",
	Usage: "plan management",
	Subcommands: cli.Commands{
		&cli.Command{
			Name:  "create",
			Usage: "create plan `PLAN_NAME`",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "module",
					Usage:    "Create module named `MODULE_NAME`",
					Required: false,
					Value:    "github.com/your/module/name",
				},
			},
			Action: createCommand,
		},
		&cli.Command{
			Name:   "list",
			Usage:  "enumerate all test cases known to the client",
			Action: listCommand,
		},
	},
}

func createCommand(c *cli.Context) error {
	if c.Args().Len() != 1 {
		return errors.New("missing reuired argument PLAN_NAME")
	}

	cfg := &config.EnvConfig{}
	if err := cfg.Load(); err != nil {
		return err
	}

	pdir := filepath.Join(cfg.Dirs().Plans(), c.Args().First())
	_, err := git.PlainInit(pdir, false)
	if err != nil {
		return err
	}

	fmap := map[string]string{
		"manifest.toml": TEMPLATE_MANIFEST_TOML,
		"main.go":       TEMPLATE_MAIN_GO,
		"go.mod":        TEMPLATE_GO_MOD,
	}

	for fn, ts := range fmap {
		tmpl, err := template.New(fn).Parse(ts)
		if err != nil {
			return err
		}
		f, err := os.Create(filepath.Join(pdir, tmpl.Name()))
		if err != nil {
			return err
		}
		tmpl.Execute(f, c.String("module"))
		f.Close()
	}
	return nil
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
