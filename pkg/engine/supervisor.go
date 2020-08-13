package engine

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/logrusorgru/aurora"
	"github.com/otiai10/copy"
	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/config"
	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/rpc"
	"github.com/testground/testground/pkg/task"
	"golang.org/x/sync/errgroup"
	"path/filepath"
	"strings"
	"time"
)

type RunInput struct {
	*api.RunRequest
	Sources *api.UnpackedSources
}

type BuildInput struct {
	*api.BuildRequest
	Sources *api.UnpackedSources
}

func (e *Engine) startSupervisor(workers int) {
	for i := 0; i < workers; i++ {
		go e.worker(i)
	}
}

func (e *Engine) worker(n int) {
	logging.S().Infow("supervisor worker started", "worker_id", n)

	var (
		store = e.TaskStorage()
		queue = e.TaskQueue()
	)

	for {
		tsk, err := queue.Pop()
		if err == task.ErrQueueEmpty {
			time.Sleep(time.Second)
			continue
		}

		ctx, _ := context.WithTimeout(context.Background(), time.Minute*30)

		err = store.AppendTaskState(tsk.ID, task.StateProcessing)
		if err != nil {
			logging.S().Errorw("could not update task status", "err", err)
		}
		logging.S().Infow("worker processing task", "worker_id", n, "task_id", tsk.ID)

		var data interface{}

		switch tsk.Type {
		case task.TaskRun:
			data, err = e.doRun(ctx, tsk.ID, tsk.Input.(*RunInput))
		case task.TaskBuild:
			data, err = e.doBuild(ctx, tsk.Input.(*BuildInput))
		default:
			// wut
		}

		err = store.MarkCompleted(tsk.ID, err, data)
		if err != nil {
			logging.S().Errorw("could not update task status", "err", err)
		}
		logging.S().Infow("worker completed task", "worker_id", n, "task_id", tsk.ID)
	}
}

func (e *Engine) doBuild(ctx context.Context, input *BuildInput) ([]*api.BuildOutput, error) {
	sources := input.Sources
	comp, err := input.Composition.PrepareForBuild(&input.Manifest)
	if err != nil {
		return nil, err
	}

	if err := comp.ValidateForBuild(); err != nil {
		return nil, fmt.Errorf("invalid composition: %w", err)
	}

	// TODO: ???
	ow := rpc.NewStdoutWriter()

	var (
		plan    = comp.Global.Plan
		builder = comp.Global.Builder
	)

	// Find the builder.
	bm, ok := e.builders[builder]
	if !ok {
		return nil, fmt.Errorf("unrecognized builder: %s", builder)
	}

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

	// This var compiles all configurations to coalesce.
	//
	// Precedence (highest to lowest):
	//
	//  1. CLI --run-param, --build-param flags.
	//  2. .env.toml.
	//  3. Builder defaults (applied by the builder itself, nothing to do here).
	//
	var cfg config.CoalescedConfig

	// 2. Get the env config for the builder.
	cfg = cfg.Append(e.envcfg.Builders[builder])

	// 1. Get overrides from the CLI.
	cfg = cfg.Append(comp.Global.BuildConfig)

	// Coalesce all configurations and deserialize into the config type
	// mandated by the builder.
	obj, err := cfg.CoalesceIntoType(bm.ConfigType())
	if err != nil {
		return nil, fmt.Errorf("error while coalescing configuration values: %w", err)
	}

	var (
		// no need to synchronise access, as each goroutine will write its
		// response in its index.
		ress   = make([]*api.BuildOutput, len(comp.Groups))
		errgrp = errgroup.Group{}
		cancel context.CancelFunc
	)

	// obtain an explicitly cancellable context so we can stop build jobs if
	// something fails.
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()

	// traverse groups, indexing them by the unique build key and remembering their position.
	uniq := make(map[string][]int, len(comp.Groups))
	for idx, g := range comp.Groups {
		k := g.Build.BuildKey()
		uniq[k] = append(uniq[k], idx)
	}

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
	var cnt int
	for key, idxs := range uniq {
		idxs := idxs
		key := key // capture

		src := finalSources[cnt]
		cnt++

		errgrp.Go(func() (err error) {
			// All groups are identical for the sake of building, so pick the first one.
			grp := comp.Groups[idxs[0]]

			// Pluck all IDs from the groups this build artifact is for.
			grpids := make([]string, 0, len(idxs))
			for _, idx := range idxs {
				grpids = append(grpids, comp.Groups[idx].ID)
			}

			ow.Infow("performing build for groups", "plan", plan, "groups", grpids, "builder", builder)

			deps := make(map[string]api.DependencyTarget, len(grp.Build.Dependencies))

			for _, dep := range grp.Build.Dependencies {
				deps[dep.Module] = api.DependencyTarget{
					Target:  dep.Target,
					Version: dep.Version,
				}
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

			res, err := bm.Build(ctx, in, ow)
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
	if err := errgrp.Wait(); err != nil {
		return nil, err
	}

	return ress, nil
}

func (e *Engine) doRun(ctx context.Context, id string, input *RunInput) (*api.RunOutput, error) {
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
		})
		if err != nil {
			return nil, err
		}

		// Populate the returned build IDs.
		for i, groupIdx := range input.BuildGroups {
			g := input.Composition.Groups[groupIdx]
			g.Run.Artifact = bout[i].ArtifactPath
		}

		// TODO: comp must be stored so can be used with --write-artifacts --wait
	}

	comp, err := input.Composition.PrepareForRun(&input.Manifest)
	if err != nil {
		return nil, err
	}

	if err := comp.ValidateForRun(); err != nil {
		return nil, err
	}

	// TODO: store the data somewhere, right? The results already have the output, should we discard?
	ow := rpc.NewStdoutWriter()

	var (
		plan   = comp.Global.Plan
		tcase  = comp.Global.Case
		runner = comp.Global.Runner
	)

	// Get the runner.
	run := e.runners[runner]

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
	cfg = cfg.Append(e.envcfg.Runners[runner])

	// 1. Get overrides from the composition.
	cfg = cfg.Append(comp.Global.RunConfig)

	// Coalesce all configurations and deserialize into the config type
	// mandated by the runner.
	obj, err := cfg.CoalesceIntoType(run.ConfigType())
	if err != nil {
		return nil, fmt.Errorf("error while coalescing configuration values: %w", err)
	}

	in := api.RunInput{
		RunID:          id,
		EnvConfig:      *e.envcfg,
		RunnerConfig:   obj,
		TestPlan:       plan,
		TestCase:       tcase,
		TotalInstances: int(comp.Global.TotalInstances),
		Groups:         make([]*api.RunGroup, 0, len(comp.Groups)),
	}

	// Trigger a build for each group, and wait until all of them are done.
	for _, grp := range comp.Groups {
		g := &api.RunGroup{
			ID:           grp.ID,
			Instances:    int(grp.CalculatedInstanceCount()),
			ArtifactPath: grp.Run.Artifact,
			Parameters:   grp.Run.TestParams,
			Resources:    grp.Resources,
		}

		in.Groups = append(in.Groups, g)
	}

	out, err := run.Run(ctx, &in, ow)
	if err == nil {
		ow.Infow("run finished successfully", "run_id", id, "plan", plan, "case", tcase, "runner", runner, "instances", in.TotalInstances)
	} else if errors.Is(err, context.Canceled) {
		ow.Infow("run canceled", "run_id", id, "plan", plan, "case", tcase, "runner", runner, "instances", in.TotalInstances)
	} else {
		ow.Warnw("run finished in error", "run_id", id, "plan", plan, "case", tcase, "runner", runner, "instances", in.TotalInstances, "error", err)
	}

	return &api.RunOutput{
		RunID:       out.RunID,
		Composition: input.Composition,
	}, err
}
