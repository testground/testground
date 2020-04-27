package cmd

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"text/template"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/config"

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
			Name:  "import",
			Usage: "import [--git] repo",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:     "git",
					Required: false,
					Value:    false,
				},
			},
			Action: importCommand,
		},
		&cli.Command{
			Name:  "rm",
			Usage: "rm [--yes] `LOCAL_REPO`",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:     "yes",
					Required: false,
					Value:    false,
				},
			},
			Action: rmCommand,
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
		err = tmpl.Execute(f, c.String("module"))
		if err != nil {
			return err
		}
		f.Close()
	}
	return nil
}

func importCommand(c *cli.Context) error {
	if c.Args().Len() != 2 {
		return errors.New("this command requires two arguments. DEST, SOURCE")
	}

	cfg := &config.EnvConfig{}
	if err := cfg.Load(); err != nil {
		return err
	}

	source := c.Args().Get(1)
	dest := filepath.Join(cfg.Dirs().Plans(), c.Args().Get(0))

	// Use git to clone. Any scheme supported by git is acceptable.
	if c.Bool("git") {
		return clonePlan(dest, source)
	}

	// not using git, simply symlink the directory. Remove the file:// scheme if it is included.
	parsed, err := url.Parse(source)
	if err != nil {
		return err
	}

	var srcPath string
	switch parsed.Scheme {
	case "file":
		srcPath = parsed.Path
	case "":
		srcPath = source
	default:
		return fmt.Errorf("unknown scheme %s for local files. did you forget to pass --git?", parsed.Scheme)
	}

	return symlinkPlan(dest, srcPath)
}

func symlinkPlan(dst, src string) error {
	abs, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	ev, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return err
	}
	return os.Symlink(ev, dst)
}

func clonePlan(dst, src string) error {

	cloneOpts := git.CloneOptions{
		URL: src,
	}

	_, err := git.PlainClone(dst, false, &cloneOpts)
	return err
}

func rmCommand(c *cli.Context) error {
	if c.Args().Len() != 1 {
		return errors.New("missing required argument PLAN_DIR")
	}

	cfg := &config.EnvConfig{}
	if err := cfg.Load(); err != nil {
		return err
	}

	if c.Bool("yes") {
		return os.RemoveAll(filepath.Join(cfg.Dirs().Plans(), c.Args().First()))
	}
	fmt.Println("really delete? pass --yes flag if you are sure.")
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
