package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/config"

	ttmpl "github.com/testground/plan-templates/templates"

	"github.com/BurntSushi/toml"
	"github.com/go-git/go-git/v5"
	gitcfg "github.com/go-git/go-git/v5/config"
	"github.com/mattn/go-zglob"
	"github.com/urfave/cli/v2"
	"github.com/whilp/git-urls"
)

var PlanCommand = cli.Command{
	Name:  "plan",
	Usage: "plan management",
	Subcommands: cli.Commands{
		&cli.Command{
			Name:  "create",
			Usage: "create a plan named `PLAN_NAME`",
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
				&cli.StringFlag{
					Name:     "plan",
					Aliases:  []string{"p"},
					Usage:    "specifies the name of the plan to create",
					Required: true,
				},
			},
			Action: createCommand,
		},
		&cli.Command{
			Name:  "import",
			Usage: "import a plan from the local filesystem or git repository",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "source",
					Usage:    "specifies the source of the plan to be imported, can be git or local.",
					Required: true,
				},
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
				&cli.StringFlag{
					Name:     "plan",
					Aliases:  []string{"p"},
					Usage:    "specifies the name of the plan to create",
					Required: true,
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

// These are the variables used for executing templates during `testground plan create`
// TODO (cory) Make these specific to the target lang?
type templateVars struct {
	Name   string
	Module string
}

func createCommand(c *cli.Context) error {
	cfg := &config.EnvConfig{}
	if err := cfg.Load(); err != nil {
		return err
	}

	plan_name := c.String("plan")
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

	// Get file templates for the supplied target lang
	asset_path := fmt.Sprintf("/%s-templates", target_lang)
	var tset ttmpl.TemplateSet
	err = ttmpl.Fill(asset_path, &tset)
	if err != nil {
		return err
	}

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
	cfg := &config.EnvConfig{}
	if err := cfg.Load(); err != nil {
		return err
	}

	source := c.String("source")

	parsed, err := giturls.Parse(source)
	if err != nil {
		return err
	}

	var importer func(string, string) error

	// determine the destation, either from flag or intuited from the soruce.
	baseDest := c.String("name")
	if baseDest == "" {
		baseDest = filepath.Base(parsed.Path)
	}
	dstPath := filepath.Join(cfg.Dirs().Home(), "plans", baseDest)

	// Use git to clone. Any scheme supported by git is acceptable.
	if c.Bool("git") {
		// Remove the '.git' from the end, if there is one, then git clone.
		dstPath = strings.TrimSuffix(dstPath, ".git")
		importer = clonePlan
	} else {

		// not using git, simply symlink the directory. Remove the file:// scheme if it is included.
		switch parsed.Scheme {
		case "file":
			source = parsed.Path
		case "":
			// this is what we expect without file://; do nothing
		default:
			return fmt.Errorf("unknown scheme %s for local files. did you forget to pass --git?", parsed.Scheme)
		}
		importer = symlinkPlan
	}

	err = importer(dstPath, source)
	if err == nil {
		fmt.Println("imported plans:")
		printPlans(cfg, dstPath)
	}
	return err
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
	fmt.Printf("created symlink %s -> %s\n", dst, src)
	return os.Symlink(ev, dst)
}

func clonePlan(dst, src string) error {

	cloneOpts := git.CloneOptions{
		URL:      src,
		Progress: os.Stderr,
	}

	_, err := git.PlainClone(dst, false, &cloneOpts)
	if err != nil {
		msg := `could not clone %s.
please double-check the git source is correct.
1. the remote repository may not exist
2. the local directory may not be empty.
3. the permissions over the given transport (ssh, git, https, etc..) may be restricted.
4. if using the SSH transport, double-check your ssh-agent is running with private keys added.
this is the error message I received:

%v
`
		return fmt.Errorf(msg, cloneOpts.URL, err)
	}
	fmt.Printf("cloned plan %s -> %s\n", dst, src)
	return nil
}

func rmCommand(c *cli.Context) error {
	cfg := &config.EnvConfig{}
	if err := cfg.Load(); err != nil {
		return err
	}

	if c.Bool("yes") {
		return os.RemoveAll(filepath.Join(cfg.Dirs().Plans(), c.String("plan")))
	}
	fmt.Println("really delete? pass --yes flag if you are sure.")
	return nil
}

func listCommand(c *cli.Context) error {
	cfg := &config.EnvConfig{}
	if err := cfg.Load(); err != nil {
		return err
	}
	return printPlans(cfg, cfg.Dirs().Plans())

}

func printPlans(cfg *config.EnvConfig, rootDir string) error {
	manifests, err := zglob.GlobFollowSymlinks(filepath.Join(rootDir, "**", "manifest.toml"))
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
