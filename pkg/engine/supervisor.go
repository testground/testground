package engine

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/logrusorgru/aurora"
	"github.com/otiai10/copy"
	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/config"
	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/rpc"
	"github.com/testground/testground/pkg/runner"
	"github.com/testground/testground/pkg/task"
	"golang.org/x/sync/errgroup"
)

type RunInput struct {
	*api.RunRequest
	Sources *api.UnpackedSources
}

type BuildInput struct {
	*api.BuildRequest
	Sources *api.UnpackedSources
}

func (e *Engine) addSignal(id string, ch chan int) {
	e.signalsLk.Lock()
	e.signals[id] = ch
	e.signalsLk.Unlock()
}

func (e *Engine) deleteSignal(id string) {
	e.signalsLk.Lock()
	delete(e.signals, id)
	e.signalsLk.Unlock()
}

func (e *Engine) worker(n int) {
	logging.S().Infow("supervisor worker started", "worker_id", n)

	taskTimeout := 10 * time.Minute
	if e.EnvConfig().Daemon.Scheduler.TaskTimeoutMin != 0 {
		taskTimeout = time.Duration(e.EnvConfig().Daemon.Scheduler.TaskTimeoutMin) * time.Minute
	}

	for {
		tsk, err := e.queue.Pop()
		if err == task.ErrQueueEmpty {
			time.Sleep(time.Second)
			continue
		}

		if err != nil {
			logging.S().Errorw("error while popping task from the queue", "err", err)
			continue
		}

		func() {
			ctx, cancel := context.WithTimeout(context.Background(), taskTimeout)
			defer cancel()

			ch := make(chan int)
			e.addSignal(tsk.ID, ch)

			go func() {
				select {
				case <-ch:
					e.deleteSignal(tsk.ID)
					cancel()
				case <-ctx.Done():
					return
				}
			}()

			tsk.States = append(tsk.States, task.DatedState{
				State:   task.StateProcessing,
				Created: time.Now().UTC(),
			})
			err = e.store.PersistProcessing(tsk)
			if err != nil {
				logging.S().Errorw("could not persist task", "err", err)
			}
			logging.S().Infow("worker processing task", "worker_id", n, "task_id", tsk.ID)
			err = e.postStatusToGithub(tsk)
			if err != nil {
				logging.S().Errorw("could not post status to github", "err", err)
			}

			// Create a packing directory under the work dir.
			file := filepath.Join(e.EnvConfig().Dirs().Daemon(), tsk.ID+".out")
			f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				logging.S().Errorw("could not create stop log", "err", err)
				return
			}
			defer f.Close()

			ow := rpc.NewFileOutputWriter(f)

			var result interface{}
			var errTask error

			switch tsk.Type {
			case task.TypeRun:
				var res *api.RunOutput
				res, errTask = e.doRun(ctx, tsk.ID, tsk.Input.(*RunInput), ow)

				if errTask != nil {
					errTask = &TaskExecutionError{TaskType: string(tsk.Type), WrappedErr: errTask}
					logging.S().Errorw("doRun returned err", "err", errTask)
				}

				if res != nil {
					result = res.Result
					tsk.Composition = res.Composition
				}
			case task.TypeBuild:
				var res []*api.BuildOutput
				res, errTask = e.doBuild(ctx, tsk.Input.(*BuildInput), ow)
				if errTask != nil {
					errTask = &TaskExecutionError{TaskType: string(tsk.Type), WrappedErr: errTask}
					logging.S().Errorw("doBuild returned err", "err", errTask)
				}

				if res != nil {
					var artifactPaths []string
					for _, ap := range res {
						artifactPaths = append(artifactPaths, ap.ArtifactPath)
					}
					result = artifactPaths
				}

			default:
				logging.S().Errorw("unknown task type", "type", tsk.Type)
				return
			}

			newState := task.DatedState{
				Created: time.Now().UTC(),
				State:   task.StateComplete,
			}
			if errTask != nil {
				tsk.Error = errTask.Error()

				var e *TaskExecutionError
				if errors.As(errTask, &e) || errors.Is(errTask, context.Canceled) {
					newState.State = task.StateCanceled
					logging.S().Errorw("task cancelled due to error", "err", errTask)
				} else {
					logging.S().Infow("Task encountered error, but was not canceled.")
				}
			}

			tsk.States = append(tsk.States, newState)
			tsk.Result = result

			err = e.store.PersistProcessing(tsk)
			if err != nil {
				logging.S().Errorw("could not persist task", "err", err)
				return
			}

			err = e.store.ArchiveTask(tsk)
			if err != nil {
				logging.S().Errorw("could not archive task", "err", err)
				return
			}

			err = e.postStatusToSlack(tsk)
			if err != nil {
				logging.S().Errorw("could not send status to slack", "err", err)
			}
			err = e.postStatusToGithub(tsk)
			if err != nil {
				logging.S().Errorw("could not post status to github", "err", err)
			}

			e.deleteSignal(tsk.ID)
			logging.S().Infow("worker completed task", "worker_id", n, "task_id", tsk.ID)
		}()
	}
}

func (e *Engine) postStatusToGithub(tsk *task.Task) error {
	if e.envcfg.Daemon.GithubRepoStatusToken == "" {
		return nil
	}

	if !tsk.CreatedByCI() {
		return nil
	}

	ownerrepo := strings.Split(tsk.CreatedBy.Repo, "/")
	owner := ownerrepo[0]
	repo := ownerrepo[1]
	hash := tsk.CreatedBy.Commit

	result, ok := tsk.Result.(*runner.Result)
	if !ok {
		return errors.New("can't post to github: task result is not from k8s")
	}

	var msg, state string

	switch tsk.State().State {
	case task.StateProcessing:
		msg = "TaaS is running your plan"
		state = "pending"
	case task.StateComplete:
		switch result.Outcome {
		case task.OutcomeSuccess:
			msg = "Testplan run succeeded!"
			state = "success"
		case task.OutcomeCanceled:
			msg = "Testplan run was canceled!"
			state = "failure"
		case task.OutcomeFailure:
			msg = "Testplan run failed!"
			state = "failure"
		case task.OutcomeUnknown:
			return errors.New("can't post update to github: task outcome is unknown")
		}
	default:
		return errors.New("can't post update to github: task state is not processing or completed")
	}

	cl := &http.Client{Timeout: time.Second * 10}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/statuses/%s", owner, repo, hash)

	plan := tsk.Plan + "/" + tsk.Case
	payload := fmt.Sprintf(`{"state":"%s","target_url":"https://ci.testground.ipfs.team/tasks","description":"%s","context":"taas/%s"}`, state, msg, plan)

	body := strings.NewReader(payload)

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", "Basic "+e.envcfg.Daemon.GithubRepoStatusToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	res, err := cl.Do(req)
	if err != nil {
		return err
	}

	res.Body.Close()

	return nil
}

func (e *Engine) postStatusToSlack(tsk *task.Task) error {
	if e.envcfg.Daemon.SlackWebhookURL == "" {
		return nil
	}

	result, ok := tsk.Result.(*runner.Result)
	if !ok {
		return nil
	}

	payload := fmt.Sprintf(`{"text":"<https://ci.testground.ipfs.team/tasks#taskID_%s|%s> *%s* run completed"}`, tsk.ID, tsk.ID, tsk.Name())

	switch result.Outcome {
	case task.OutcomeSuccess:
		payload = fmt.Sprintf(`{"text":"✅ <https://ci.testground.ipfs.team/tasks#taskID_%s|%s> *%s* run succeeded (%s) %s"}`, tsk.ID, tsk.ID, tsk.Name(), result, tsk.Took())
	case task.OutcomeCanceled:
		payload = fmt.Sprintf(`{"text":"⚪ <https://ci.testground.ipfs.team/tasks#taskID_%s|%s> *%s* run canceled %s ; %s"}`, tsk.ID, tsk.ID, tsk.Name(), tsk.Took(), tsk.Error)
	case task.OutcomeFailure:
		payload = fmt.Sprintf(`{"text":"❌ <https://ci.testground.ipfs.team/tasks#taskID_%s|%s> *%s* run failed (%s) %s ; %s"}`, tsk.ID, tsk.ID, tsk.Name(), result, tsk.Took(), tsk.Error)
	}

	cl := &http.Client{Timeout: time.Second * 10}
	body := strings.NewReader(payload)
	res, err := cl.Post(
		e.envcfg.Daemon.SlackWebhookURL,
		"application/json; charset=UTF-8",
		body,
	)
	if err != nil {
		return err
	}

	res.Body.Close()

	return nil
}

func (e *Engine) doBuild(ctx context.Context, input *BuildInput, ow *rpc.OutputWriter) ([]*api.BuildOutput, error) {
	sources := input.Sources
	comp, err := input.Composition.PrepareForBuild(&input.Manifest)

	if err != nil {
		return nil, err
	}

	if err := comp.ValidateForBuild(); err != nil {
		return nil, fmt.Errorf("invalid composition: %w", err)
	}

	var (
		plan = clean(comp.Global.Plan)
	)

	// Validate builders we use
	usedBuilders := comp.ListBuilders()

	for _, b := range usedBuilders {
		_, ok := e.builders[b]

		if !ok {
			return nil, fmt.Errorf("unrecognized builder: %s", b)
		}
	}

	// Call healthcheck on the builders
	for _, b := range usedBuilders {
		bm := e.builders[b]

		// Call the healthcheck routine if the runner supports it, with fix=true.
		if hc, ok := bm.(api.Healthchecker); ok {
			ow.Info("performing healthcheck on builder")

			if rep, err := hc.Healthcheck(ctx, e, ow, true); err != nil {
				return nil, fmt.Errorf("healthcheck and fix errored: %w", err)
			} else if !rep.FixesSucceeded() {
				return nil, fmt.Errorf("healthcheck fixes failed; aborting:\n%s", rep)
			} else if !rep.ChecksSucceeded() {
				ow.Warnf(aurora.Bold(aurora.Yellow("some healthchecks failed, but continuing")).String())
			} else {
				ow.Infof(aurora.Bold(aurora.Green("healthcheck: ok")).String())
			}
		}
	}

	// Builders are ready, let's go.
	var (
		// no need to synchronise access, as each goroutine will write its
		// response in its index.
		ress   = make([]*api.BuildOutput, len(comp.Groups))
		cancel context.CancelFunc
	)

	// obtain an explicitly cancellable context so we can stop build jobs if
	// something fails.
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()

	// traverse groups, indexing them by the unique build key and remembering their position.
	uniq := make(map[string][]int, len(comp.Groups))
	for idx, g := range comp.Groups {
		// NOTE: why do we even need this and don't rely on docker layer caching?
		k := g.BuildKey()
		uniq[k] = append(uniq[k], idx)
	}

	// prepare sources
	var finalSources []*api.UnpackedSources
	if uniqcnt := len(uniq); uniqcnt == 1 {
		finalSources = []*api.UnpackedSources{sources}
	} else {
		finalSources = make([]*api.UnpackedSources, uniqcnt)

		for i := 0; i < uniqcnt; i++ {
			dst := fmt.Sprintf("%s-%d", strings.TrimSuffix(sources.BaseDir, "/"), i)
			if err := copy.Copy(sources.BaseDir, dst); err != nil {
				return nil, fmt.Errorf("failed to create unique source directories for multiple build jobs: %w", err)
			}
			src := &api.UnpackedSources{
				BaseDir: dst,
				PlanDir: filepath.Join(dst, filepath.Base(sources.PlanDir)),
			}
			if sources.SDKDir != "" {
				src.SDKDir = filepath.Join(dst, filepath.Base(sources.SDKDir))
			}
			if sources.ExtraDir != "" {
				src.ExtraDir = filepath.Join(dst, filepath.Base(sources.ExtraDir))
			}
			finalSources[i] = src
		}
	}

	// Trigger a build job for each unique build, and wait until all of them are
	// done, mapping the build artifacts back to the original group positions in
	// the response.
	errGroup, errGroupCtx := errgroup.WithContext(ctx)

	concurrentBuilds := comp.Global.ConcurrentBuilds
	if concurrentBuilds == 0 {
		concurrentBuilds = -1
	}
	errGroup.SetLimit(concurrentBuilds)

	var cnt int
	for key, idxs := range uniq {
		idxs := idxs
		key := key // capture

		src := finalSources[cnt]
		cnt++

		errGroup.Go(func() (err error) {
			// Every Group in `idxs`` have the same build key. They are identitical when it comes to build,
			// so it's safe to use the first one to build them all.
			grp := comp.Groups[idxs[0]]

			// Pluck all IDs from the groups this build artifact is for.
			grpids := make([]string, 0, len(idxs))
			for _, idx := range idxs {
				grpids = append(grpids, comp.Groups[idx].ID)
			}

			// get the builder
			builder := grp.Builder
			bm := e.builders[builder]

			ow.Infow("performing build for groups", "plan", plan, "groups", grpids, "builder", builder)

			deps := make(map[string]api.DependencyTarget, len(grp.Build.Dependencies))

			for _, dep := range grp.Build.Dependencies {
				deps[dep.Module] = api.DependencyTarget{
					Target:  dep.Target,
					Version: dep.Version,
				}
			}

			// This var compiles all configurations to coalesce.
			//
			// Precedence (highest to lowest):
			//
			//  1. CLI --run-param, --build-param flags.
			//  2. .env.toml.
			//  3. Builder defaults (applied by the builder itself, nothing to do here).
			//
			var cfg config.CoalescedConfig
			cfg = cfg.Append(e.envcfg.Builders[builder]) // env config for the builder
			groupCfg := cfg.Append(grp.BuildConfig)      // add the group config

			// Coalesce all configurations and deserialize into the config type
			// mandated by the builder.
			obj, err := groupCfg.CoalesceIntoType(bm.ConfigType())

			if err != nil {
				return fmt.Errorf("error while coalescing configuration values: %w", err)
			}

			in := &api.BuildInput{
				BuildID:         uuid.New().String()[24:],
				EnvConfig:       *e.envcfg,
				TestPlan:        plan,
				Selectors:       grp.Build.Selectors,
				Dependencies:    deps,
				BuildConfig:     obj,
				UnpackedSources: src,
			}

			res, err := bm.Build(errGroupCtx, in, ow)
			if err != nil {
				ow.Infow("build failed", "plan", plan, "groups", grpids, "builder", builder, "error", err)
				return err
			}

			res.BuilderID = bm.ID()

			// no need for a mutex as the indices we access do not intersect
			// across goroutines.
			for _, idx := range uniq[key] {
				ress[idx] = res
			}

			ow.Infow("build succeeded", "plan", plan, "groups", grpids, "builder", builder, "artifact", res.ArtifactPath)
			return nil
		})
	}

	// Wait until all goroutines are done. If any failed, return the error.
	if err := errGroup.Wait(); err != nil {
		return nil, err
	}

	return ress, nil
}

func (e *Engine) doRun(ctx context.Context, id string, input *RunInput, ow *rpc.OutputWriter) (*api.RunOutput, error) {
	// TODO: this is hackish, let's redesign this:
	// Next should be:

	// 1. Identify the groups we're going to run from the input.RunIds parameters + missing artifacts
	// 2. Build the groups
	// 3. For every run parameter,
	//  3.1. Generate the run info
	//  3.2. Schedule it somehow.
	//  3.3 Wait and gather results
	// 4. Aggregate the results and return them.

	// For the first step let's:
	// 1. Assume every group needs to be built
	// 2. Build the groups
	// 3. Assume there is only one run
	// 	3.1. Generate the run info in a practical way
	// 4. Return as before.
	if len(input.BuildGroups) > 0 {
		bcomp, err := input.Composition.PickGroups(input.BuildGroups...)
		if err != nil {
			return nil, err
		}

		bout, err := e.doBuild(ctx, &BuildInput{
			BuildRequest: &api.BuildRequest{
				Composition: bcomp,
				Manifest:    input.Manifest,
			},
			Sources: input.Sources,
		}, ow)
		if err != nil {
			return nil, err
		}

		// Populate the returned build IDs. This is returned so the
		// client can store the composition with artifacts if they chose to.
		for i, groupIdx := range input.BuildGroups {
			g := input.Composition.Groups[groupIdx]
			g.Run.Artifact = bout[i].ArtifactPath
		}
	}

	comp, err := input.Composition.PrepareForRun(&input.Manifest)
	if err != nil {
		return nil, err
	}

	if err := comp.ValidateForRun(); err != nil {
		return nil, err
	}

	compositionUsedForRun := comp

	var (
		plan    = comp.Global.Plan
		tcase   = comp.Global.Case
		trunner = comp.Global.Runner
	)

	// Get the runner.
	run := e.runners[trunner]

	// Call the healthcheck routine if the runner supports it, with fix=true.
	if hc, ok := run.(api.Healthchecker); ok {
		ow.Info("performing healthcheck on runner")

		if rep, err := hc.Healthcheck(ctx, e, ow, true); err != nil {
			return nil, fmt.Errorf("healthcheck and fix errored: %w", err)
		} else if !rep.FixesSucceeded() {
			return nil, fmt.Errorf("healthcheck fixes failed; aborting:\n%s", rep)
		} else if !rep.ChecksSucceeded() {
			ow.Warnf(aurora.Bold(aurora.Yellow("some healthchecks failed, but continuing")).String())
		} else {
			ow.Infof(aurora.Bold(aurora.Green("healthcheck: ok")).String())
		}
	}

	// This var compiles all configurations to coalesce.
	//
	// Precedence (highest to lowest):
	//
	//  1. CLI --run-param, --build-param flags.
	//  2. .env.toml.
	//  3. Builder defaults (applied by the builder itself, nothing to do here).
	//
	var cfg config.CoalescedConfig

	// 2. Get the env config for the runner.
	cfg = cfg.Append(e.envcfg.Runners[trunner])

	var flag = e.envcfg.Runners[trunner][config.RunnerDisabledFlag]
	if flag == true {
		return nil, runner.ErrRunnerDisabled
	}

	// 1. Get overrides from the composition.
	cfg = cfg.Append(comp.Global.RunConfig)

	// Coalesce all configurations and deserialize into the config type
	// mandated by the runner.
	obj, err := cfg.CoalesceIntoType(run.ConfigType())
	if err != nil {
		return nil, fmt.Errorf("error while coalescing configuration values: %w", err)
	}

	if (len(input.RunIds) > 1) {
		// TODO: remove when we can build multiple runs
		return nil, fmt.Errorf("cannot specify multiple run ids for now")
	}

	runId := input.RunIds[0]
	framedComp, err := comp.FrameForRuns(runId);

	if err != nil {
		return nil, fmt.Errorf("error while framing composition for run: %s: %w", runId, err)
	}

	compRun := framedComp.Runs[0]

	in := api.RunInput{
		RunID:          id,
		EnvConfig:      *e.envcfg,
		RunnerConfig:   obj,
		TestPlan:       clean(plan),
		TestCase:       clean(tcase),
		TotalInstances: int(compRun.TotalInstances),
		Groups:         make([]*api.RunGroup, 0, len(compRun.Groups)),
		DisableMetrics: comp.Global.DisableMetrics,
	}

	for _, grp := range compRun.Groups {
		buildgroup, err := framedComp.GetGroup(grp.EffectiveGroupId())
		if err != nil {
			return nil, err
		}

		g := &api.RunGroup{
			ID:           grp.ID,
			Instances:    int(grp.CalculatedInstanceCount()),
			ArtifactPath: buildgroup.Run.Artifact,
			Parameters:   grp.TestParams,
			Resources:    grp.Resources,
			Profiles:     grp.Profiles,
		}

		in.Groups = append(in.Groups, g)
	}

	ow.Infow("starting run", "run_id", id, "plan", in.TestPlan, "case", in.TestCase, "runner", trunner, "instances", in.TotalInstances)
	out, err := run.Run(ctx, &in, ow)

	if err == nil {
		message := "run finished with outcome unknown"
		if out.Result != nil {
			message = fmt.Sprintf("run finished with %v", out.Result)
		}

		ow.Infow(message, "run_id", id, "plan", plan, "case", tcase, "runner", trunner, "instances", in.TotalInstances)
	} else if errors.Is(err, context.Canceled) {
		ow.Infow("run canceled", "run_id", id, "plan", plan, "case", tcase, "runner", trunner, "instances", in.TotalInstances)
	} else {
		ow.Warnw("run finished in error", "run_id", id, "plan", plan, "case", tcase, "runner", trunner, "instances", in.TotalInstances, "error", err)
	}

	if out != nil { // TODO: Make sure all runners return a value, and get rid of nil check
		out.Composition = *compositionUsedForRun
	}

	return out, err
}

func clean(name string) string {
	forbiddenChar := "/"

	name = strings.Replace(name, forbiddenChar, "-", -1)

	return name
}
