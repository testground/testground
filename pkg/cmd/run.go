package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/client"
	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/task"

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
				&cli.StringFlag{
					Name:  "run-ids",
					Usage: "run a specific run id, or a comma-separated list of run ids",
				},
				&cli.StringFlag{
					Name:  "metadata-repo",
					Usage: "repo that triggered this run",
				},
				&cli.StringFlag{
					Name:  "metadata-branch",
					Usage: "branch that triggered this run",
				},
				&cli.StringFlag{
					Name:  "metadata-commit",
					Usage: "commit that triggered this run",
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
				&cli.StringFlag{
					Name:  "metadata-repo",
					Usage: "repo that triggered this run",
				},
				&cli.StringFlag{
					Name:  "metadata-branch",
					Usage: "branch that triggered this run",
				},
				&cli.StringFlag{
					Name:  "metadata-commit",
					Usage: "commit that triggered this run",
				},
				&cli.BoolFlag{
					Name:  "disable-metrics",
					Usage: "disable metrics batching",
				},
			),
		},
	},
}

func runCompositionCmd(c *cli.Context) (err error) {
	file := c.String("file")
	if file == "" {
		return fmt.Errorf("no composition file supplied")
	}

	comp, err := loadComposition(file)

	if err != nil {
		return fmt.Errorf("failed to load composition file: %w", err)
	}

	if err = comp.ValidateForRun(); err != nil {
		return fmt.Errorf("invalid composition file: %w", err)
	}

	err = run(c, comp)
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
	return run(c, comp)
}

func run(c *cli.Context, comp *api.Composition) (err error) {
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

	// Retrieve the run ids to use.
	rawRunIds := c.String("run-ids")
	var runIds []string

	// default to all the runs in the composition
	if rawRunIds == "" {
		runIds = comp.ListRunIds()
	} else {
		runIds = strings.Split(rawRunIds, ",")
	}

	// TODO: validate run ids
	// TODO: verify run ids exists in the composition.

	// Skip artifacts if the user explicit requests it.
	// TODO: Simplify this code: empty the artifact field if required and post
	//       the composition to the daemon. The daemon will take care of identifying
	// 		 which artifacts should be built, etc.
	//		 Eventually we'll drop the BuildGroups field from the request.
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
		builder := strings.Replace(comp.Global.Builder, ":", "_", -1)
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

	// Execute!

	// Compute priority
	isCollecting := c.Bool("collect")
	isMultiple := len(runIds) > 1
	isWaiting := c.Bool("wait") || isCollecting || isMultiple

	priority := 0
	if isWaiting {
		priority = 1
	}

	// Compute compositionTarget
	compositionTarget := ""

	if file := c.String("file"); file != "" && c.Bool("write-artifacts") {
		compositionTarget = file
	}

	collectionTarget := c.String("collect-file")

	// Prepare the strategy
	strategy := MultiRunStrategy{
		CurrentRunIndex:      0,
		RunIds:               runIds,
		Composition:          comp,
		EffectiveComposition: comp,
		BaseRequest: api.RunRequest{
			BuildGroups: buildIdx,
			Priority:    priority,
			RunIds:      []string{},
			Composition: *comp,
			Manifest:    *manifest,
			CreatedBy: api.CreatedBy{
				User:   cfg.Client.User,
				Repo:   c.String("metadata-repo"),
				Branch: c.String("metadata-branch"),
				Commit: c.String("metadata-commit"),
			},
		},
		planDir:           planDir,
		sdkDir:            sdkDir,
		extraSrcs:         extraSrcs,
		isCollecting:      isCollecting,
		isWaiting:         isWaiting,
		isMultiple:        isMultiple,
		compositionTarget: compositionTarget,
		collectionTarget:  collectionTarget,
	}

	for {
		shouldContinue, err := strategy.Next(ctx, cl, c)
		if err != nil {
			return err
		}

		if !shouldContinue {
			break
		}
	}

	return nil
}

func (m *MultiRunStrategy) Next(ctx context.Context, cl *client.Client, c *cli.Context) (bool, error) {
	// Done
	if m.CurrentRunIndex >= len(m.RunIds) {
		return false, nil
	}

	taskId, err := m.CallDaemonRun(ctx, cl)
	if err != nil {
		return false, err
	}

	// We're not waiting, let's leave
	if !m.isWaiting {
		return false, nil
	}

	// Wait for the task to finish.
	tsk, err := m.WaitForTaskCompletion(ctx, cl, taskId)

	if err != nil {
		return false, err
	}

	// Process the composition
	err = m.ProcessComposition(tsk)

	if err != nil {
		return false, err
	}

	// Process the collection
	err = m.Collect(ctx, cl, tsk.ID)

	if err != nil {
		return false, err
	}

	m.CurrentRunIndex += 1
	return true, nil
}

func (m *MultiRunStrategy) CurrentRequest() api.RunRequest {
	request := m.BaseRequest
	request.RunIds = []string{m.CurrentRunId()}

	// No build groups, we are using the effective composition
	if m.CurrentRunIndex != 0 {
		request.BuildGroups = []int{}
	}

	request.Composition = *m.EffectiveComposition

	return request
}

func (m *MultiRunStrategy) CurrentRunId() string {
	return m.RunIds[m.CurrentRunIndex]
}

func (m *MultiRunStrategy) CallDaemonRun(ctx context.Context, cl *client.Client) (string, error) {
	req := m.CurrentRequest()
	resp, err := cl.Run(ctx, &req, m.planDir, m.sdkDir, m.extraSrcs)

	switch err {
	case nil:
		break // noop
	case context.Canceled:
		return "", fmt.Errorf("interrupted")
	default:
		return "", err
	}

	defer resp.Close()

	id, err := client.ParseRunResponse(resp)
	if err != nil {
		return "", err
	}

	logging.S().Infof("run is queued with ID: %s", id)
	return id, nil
}

func (m *MultiRunStrategy) WaitForTaskCompletion(ctx context.Context, cl *client.Client, taskId string) (*task.Task, error) {
	r, err := cl.Logs(ctx, &api.LogsRequest{
		TaskID:            taskId,
		Follow:            true,
		CancelWithContext: true,
	})
	if err != nil {
		return nil, err
	}
	defer r.Close()

	tsk, err := client.ParseLogsRequest(os.Stdout, r)
	if err != nil {
		return nil, err
	}

	if tsk.Error != "" {
		return nil, errors.New(tsk.Error)
	}

	logging.S().Infof("finished run with ID: %s", taskId)

	return &tsk, nil
}

func (m *MultiRunStrategy) ProcessComposition(tsk *task.Task) error {
	// for the first run, keep the composition produced by the daemon.
	if m.CurrentRunIndex == 0 {
		var composition api.Composition
		err := mapstructure.Decode(tsk.Composition, &composition)

		if err != nil {
			return err
		}

		m.EffectiveComposition = &composition
	}

	// If that's the first run and we are asking for a write artifact, store it
	if m.CurrentRunIndex == 0 && m.compositionTarget != "" {
		err := api.WriteCompositionToFile(m.EffectiveComposition, m.compositionTarget)
		if err != nil {
			return fmt.Errorf("failed to write composition file: %w", err)
		}
	}

	return nil
}

func (m *MultiRunStrategy) CurrentCollectedPath(taskId string) string {
	collectionTarget := m.collectionTarget

	if m.isMultiple {
		if collectionTarget == "" {
			collectionTarget = "multiple"
		}
		collectionTarget = fmt.Sprintf("%s-%s-%s", collectionTarget, m.CurrentRunId(), taskId)
	} else {
		if collectionTarget == "" {
			collectionTarget = taskId
		}
	}

	return fmt.Sprintf("%s.tgz", collectionTarget)
}

func (m *MultiRunStrategy) Collect(ctx context.Context, cl *client.Client, taskId string) error {
	if m.isCollecting {
		err := collect(ctx, cl, m.Composition.Global.Runner, taskId, m.CurrentCollectedPath(taskId))

		if err != nil {
			return cli.Exit(err.Error(), 3)
		}
	}

	return nil
}

type MultiRunStrategy struct {
	// Current RunID Index
	CurrentRunIndex int

	// Run IDs
	RunIds []string

	// Initial Composition
	Composition *api.Composition

	// Effective Composition used by the daemon, which contains artifacts
	EffectiveComposition *api.Composition

	// Base Request
	BaseRequest api.RunRequest

	// Collect Destination
	CollectDestination string

	// Composition Destination
	CompositionDestination string

	// Plan configuration
	planDir   string
	sdkDir    string
	extraSrcs []string

	// Flags
	isCollecting bool
	isWaiting    bool
	isMultiple   bool

	// Outputs
	compositionTarget string
	collectionTarget  string
}
