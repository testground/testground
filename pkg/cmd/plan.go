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
	gitcfg "github.com/go-git/go-git/v5/config"
	"github.com/mattn/go-zglob"
	"github.com/urfave/cli/v2"
)

var PlanCommand = cli.Command{
	Name:  "plan",
	Usage: "plan management",
	Subcommands: cli.Commands{
		&cli.Command{
			Name:      "create",
			Usage:     "create a plan named `PLAN_NAME`",
			ArgsUsage: "`PLAN_NAME`: this will be the directory in $TESTGROUND_HOME",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "remote",
					Usage:    "origin for your repository i.e. git@github.com:your/repo",
					Required: false,
				},
				&cli.StringFlag{
					Name:     "target",
					Usage:    "target language (default: go), avail: [go]",
					Required: false,
					Value:    "go",
				},
				&cli.StringFlag{
					Name:  "module",
					Usage: "module name (used for initial templating",
					Value: "github.com/your/module/name",
				},
			},
			Action: createCommand,
		},
		&cli.Command{
			Name:      "import",
			Usage:     "import a plan from the local filesystem or git repository",
			ArgsUsage: "`SOURCE`: the location of the plan to be imported",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:     "git",
					Usage:    "use git to import (default: false)",
					Required: false,
					Value:    false,
				},
				&cli.StringFlag{
					Name:     "name",
					Usage:    "override the name of the plan directory (default: automatic)",
					Required: false,
				},
			},
			Action: importCommand,
		},
		&cli.Command{
			Name:  "rm",
			Usage: "remove a plan directory from $TESTGROUND_HOME",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:     "yes",
					Usage:    "confirm removal (without this, the command does nothing)",
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
		return errors.New("this command requires one argument -- specify the plan name")
	}

	cfg := &config.EnvConfig{}
	if err := cfg.Load(); err != nil {
		return err
	}

	plan_name := c.Args().First()
	target_lang := c.String("target")
	remote := c.String("remote")
	module := c.String("module")

	pdir := filepath.Join(cfg.Dirs().Plans(), plan_name)
	repo, err := git.PlainInit(pdir, false)
	if err != nil {
		return err
	}

	if remote != "" {
		_, err := repo.CreateRemote(&gitcfg.RemoteConfig{
			Name: "origin",
			URLs: []string{remote},
		})
		if err != nil {
			return err
		}
	}

	tset := GetTemplateSet(target_lang)

	if tset == nil {
		return fmt.Errorf("unknown language target %s", target_lang)
	}

	for _, ts := range tset {
		tmpl, err := template.New(ts.Filename).Parse(ts.Template)
		if err != nil {
			return err
		}
		f, err := os.Create(filepath.Join(pdir, tmpl.Name()))
		if err != nil {
			return err
		}
		err = tmpl.Execute(f, templateVars{Name: plan_name, Module: module})
		if err != nil {
			return err
		}
		f.Close()
	}
	return nil
}

func importCommand(c *cli.Context) error {
	if c.Args().Len() != 1 {
		return errors.New("this command requires one argument, the location of the plan to import")
	}

	cfg := &config.EnvConfig{}
	if err := cfg.Load(); err != nil {
		return err
	}

	source := c.Args().Get(0)

	parsed, err := url.Parse(source)
	if err != nil {
		return err
	}

	// determine the destation, either from flag or intuited from the soruce.
	baseDest := c.String("name")
	if baseDest == "" {
		baseDest = filepath.Base(parsed.Path)
	}
	dstPath := filepath.Join(cfg.Dirs().Home(), "plans", baseDest)

	// Use git to clone. Any scheme supported by git is acceptable.
	if c.Bool("git") {
		return clonePlan(dstPath, source)
	}

	// not using git, simply symlink the directory. Remove the file:// scheme if it is included.
	var srcPath string
	switch parsed.Scheme {
	case "file":
		srcPath = parsed.Path
	case "":
		srcPath = source
	default:
		return fmt.Errorf("unknown scheme %s for local files. did you forget to pass --git?", parsed.Scheme)
	}
	return symlinkPlan(dstPath, srcPath)
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
		URL:      src,
		Progress: os.Stderr,
	}

	_, err := git.PlainClone(dst, false, &cloneOpts)
	return err
}

func rmCommand(c *cli.Context) error {
	if c.Args().Len() != 1 {
		return errors.New("this plan requires one argument, the name of the plan to remove.")
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
