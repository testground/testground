package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/config"
	"github.com/testground/testground/pkg/logging"

	ttmpl "github.com/testground/plan-templates/templates"

	"github.com/BurntSushi/toml"
	"github.com/go-git/go-git/v5"
	gitcfg "github.com/go-git/go-git/v5/config"
	"github.com/mattn/go-zglob"
	"github.com/urfave/cli/v2"
	giturls "github.com/whilp/git-urls"
)

var PlanCommand = cli.Command{
	Name:  "plan",
	Usage: "manage the plans known to the client",
	Subcommands: cli.Commands{
		&cli.Command{
			Name:  "create",
			Usage: "creates a new test plan, using a template from github.com/testground/plan-templates",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "remote",
					Usage:    "`URL` of the repo where this plan will be hosted i.e. git@github.com:your/repo",
					Required: false,
				},
				&cli.StringFlag{
					Name:     "target",
					Usage:    "use template for target `LANGUAGE`; values: go",
					Required: false,
					Value:    "go",
				},
				&cli.StringFlag{
					Name:  "module",
					Usage: "set `MODULE_NAME`, used for initial templating",
					Value: "github.com/your/module/name",
				},
				&cli.StringFlag{
					Name:     "plan",
					Aliases:  []string{"p"},
					Usage:    "set `NAME` of the plan to create",
					Required: true,
				},
			},
			Action: createCommand,
		},
		&cli.Command{
			Name:  "import",
			Usage: "import a plan from the local filesystem or a git repository into $TESTGROUND_HOME",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "from",
					Usage:    "the source `URL` of the plan to be imported; either a path, or a Git remote",
					Required: true,
				},
				&cli.BoolFlag{
					Name:     "git",
					Usage:    "import from a git repository",
					Required: false,
					Value:    false,
				},
				&cli.StringFlag{
					Name:        "name",
					Usage:       "override the `NAME` of the plan directory",
					Required:    false,
					DefaultText: "automatic",
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
			Usage:  "enumerate all test plans or test cases known to the client",
			Action: listCommand,
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:  "testcases",
					Usage: "display testcases",
				},
			},
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

	var (
		planName   = c.String("plan")
		targetLang = c.String("target")
		remote     = c.String("remote")
		module     = c.String("module")
	)

	pdir := filepath.Join(cfg.Dirs().Plans(), planName)
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
	assetPath := fmt.Sprintf("/%s-templates", targetLang)
	var tset ttmpl.TemplateSet
	err = ttmpl.Fill(assetPath, &tset)
	if err != nil {
		return err
	}

	if tset == nil {
		return fmt.Errorf("unknown language target %s", targetLang)
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
		err = tmpl.Execute(f, templateVars{Name: planName, Module: module})
		if err != nil {
			return err
		}
		f.Close()
	}
	fmt.Println("new test plan created under:", pdir)
	return nil
}

func importCommand(c *cli.Context) error {
	cfg := &config.EnvConfig{}
	if err := cfg.Load(); err != nil {
		return err
	}

	from := c.String("from")

	parsed, err := giturls.Parse(from)
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

	// check if path exists
	if _, err := os.Stat(dstPath); !os.IsNotExist(err) {
		logging.S().Warnw("destination dir already exists", "path", dstPath)
		return nil
	}

	// Use git to clone. Any scheme supported by git is acceptable.
	if c.Bool("git") {
		// Remove the '.git' from the end, if there is one, then git clone.
		dstPath = strings.TrimSuffix(dstPath, ".git")
		importer = clonePlan
	} else {

		// not using git, simply symlink the directory. Remove the file:// scheme if it is included.
		switch parsed.Scheme {
		case "file":
			from = parsed.Path
		case "":
			// this is what we expect without file://; do nothing
		default:
			return fmt.Errorf("unknown scheme %s for local files. did you forget to pass --git?", parsed.Scheme)
		}
		importer = symlinkPlan
	}

	err = importer(dstPath, from)
	if err == nil {
		fmt.Println("imported plans:")
		_ = printPlans(cfg, dstPath, true)
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
1. the remote repository may not exist.
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
		pdir := filepath.Join(cfg.Dirs().Plans(), c.String("plan"))
		err := os.RemoveAll(pdir)
		if err != nil {
			return err
		}
		fmt.Printf("plan at %s removed.\n", pdir)
		return nil
	}

	fmt.Println("really delete? pass --yes flag to confirm.")
	return nil
}

func listCommand(c *cli.Context) error {
	cfg := &config.EnvConfig{}
	if err := cfg.Load(); err != nil {
		return err
	}
	return printPlans(cfg, cfg.Dirs().Plans(), c.Bool("testcases"))

}

func printPlans(cfg *config.EnvConfig, rootDir string, testcases bool) error {
	manifests, err := zglob.GlobFollowSymlinks(filepath.Join(rootDir, "**", "manifest.toml"))
	if err != nil {
		return fmt.Errorf("failed to discover test plans under %s: %w", cfg.Dirs().Plans(), err)
	}

	tw := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	defer tw.Flush()

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

		if testcases {
			for _, tc := range manifest.TestCases {
				_, _ = fmt.Fprintf(tw, "%s\t%s\n", plan, tc.Name)
			}
		} else {
			_, _ = fmt.Fprintln(tw, plan)
		}
	}

	return nil
}
