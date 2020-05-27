package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/build"
	"github.com/testground/testground/pkg/config"
	"github.com/testground/testground/pkg/rpc"
	"github.com/testground/testground/pkg/runner"

	"github.com/google/uuid"
	"github.com/logrusorgru/aurora"
	"golang.org/x/sync/errgroup"
)

// AllBuilders enumerates all builders known to the system.
var AllBuilders = []api.Builder{
	&build.DockerGoBuilder{},
	&build.ExecGoBuilder{},
	&build.DockerGenericBuilder{},
}

// AllRunners enumerates all runners known to the system.
var AllRunners = []api.Runner{
	&runner.LocalDockerRunner{},
	&runner.LocalExecutableRunner{},
	&runner.ClusterSwarmRunner{},
	&runner.ClusterK8sRunner{},
}

// Engine is the central runtime object of the system. It knows about all test
// plans, builders, and runners. It is supposed to be instantiated as a
// singleton in all runtimes, whether the testground is run as a CLI tool, or as
// a daemon. In the latter mode, the GitHub bridge will trigger commands and
// perform queries on the Engine.
//
// TODO: the Engine should also centralise all system state and make it
// queriable, e.g. what tests are running, or have run, such that we can easily
// query test plans that ran for a particular commit of an upstream.
type Engine struct {
	lk sync.RWMutex
	// builders binds builders to their identifying key.
	builders map[string]api.Builder
	// runners binds runners to their identifying key.
	runners map[string]api.Runner
	envcfg  *config.EnvConfig
	ctx     context.Context
}

var _ api.Engine = (*Engine)(nil)

type EngineConfig struct {
	Builders  []api.Builder
	Runners   []api.Runner
	EnvConfig *config.EnvConfig
}

func NewEngine(cfg *EngineConfig) (*Engine, error) {
	e := &Engine{
		builders: make(map[string]api.Builder, len(cfg.Builders)),
		runners:  make(map[string]api.Runner, len(cfg.Runners)),
		envcfg:   cfg.EnvConfig,
		ctx:      context.Background(),
	}

	for _, b := range cfg.Builders {
		e.builders[b.ID()] = b
	}

	for _, r := range cfg.Runners {
		e.runners[r.ID()] = r
	}

	return e, nil
}

func NewDefaultEngine(ecfg *config.EnvConfig) (*Engine, error) {
	cfg := &EngineConfig{
		Builders:  AllBuilders,
		Runners:   AllRunners,
		EnvConfig: ecfg,
	}

	e, err := NewEngine(cfg)
	if err != nil {
		return nil, err
	}

	return e, nil
}

func (e *Engine) BuilderByName(name string) (api.Builder, bool) {
	e.lk.RLock()
	defer e.lk.RUnlock()

	m, ok := e.builders[name]
	return m, ok
}

func (e *Engine) RunnerByName(name string) (api.Runner, bool) {
	e.lk.RLock()
	defer e.lk.RUnlock()

	m, ok := e.runners[name]
	return m, ok
}

func (e *Engine) ListBuilders() map[string]api.Builder {
	e.lk.RLock()
	defer e.lk.RUnlock()

	m := make(map[string]api.Builder, len(e.builders))
	for k, v := range e.builders {
		m[k] = v
	}
	return m
}

func (e *Engine) ListRunners() map[string]api.Runner {
	e.lk.RLock()
	defer e.lk.RUnlock()

	m := make(map[string]api.Runner, len(e.runners))
	for k, v := range e.runners {
		m[k] = v
	}
	return m
}

func (e *Engine) DoBuild(ctx context.Context, comp *api.Composition, basesrc string, plansrc string, sdksrc string, ow *rpc.OutputWriter) ([]*api.BuildOutput, error) {
	if err := comp.ValidateForBuild(); err != nil {
		return nil, fmt.Errorf("invalid composition: %w", err)
	}

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

	// Coalesce all configurations and deserialise into the config type
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

	// Trigger a build job for each unique build, and wait until all of them are
	// done, mapping the build artifacts back to the original group positions in
	// the response.
	for key, idxs := range uniq {
		idxs := idxs
		key := key // capture

		errgrp.Go(func() (err error) {
			// All groups are identical for the sake of building, so pick the first one.
			grp := comp.Groups[idxs[0]]

			// Pluck all IDs from the groups this build artifact is for.
			grpids := make([]string, 0, len(idxs))
			for _, idx := range idxs {
				grpids = append(grpids, comp.Groups[idx].ID)
			}

			ow.Infow("performing build for groups", "plan", plan, "groups", grpids, "builder", builder)

			in := &api.BuildInput{
				BuildID:         uuid.New().String()[24:],
				EnvConfig:       *e.envcfg,
				TestPlan:        plan,
				Selectors:       grp.Build.Selectors,
				Dependencies:    grp.Build.Dependencies.AsMap(),
				BuildConfig:     obj,
				BaseSrcPath:     basesrc,
				TestPlanSrcPath: plansrc,
				SDKSrcPath:      sdksrc,
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

func (e *Engine) DoRun(ctx context.Context, comp *api.Composition, ow *rpc.OutputWriter) (*api.RunOutput, error) {
	if err := comp.ValidateForRun(); err != nil {
		return nil, fmt.Errorf("invalid composition: %w", err)
	}

	var (
		plan    = comp.Global.Plan
		tcase   = comp.Global.Case
		builder = comp.Global.Builder
		runner  = comp.Global.Runner
	)

	// Get the runner.
	run, ok := e.runners[runner]
	if !ok {
		return nil, fmt.Errorf("unknown runner: %s", runner)
	}

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

	// Check if builder and runner are compatible
	if !stringInSlice(comp.Global.Builder, run.CompatibleBuilders()) {
		return nil, fmt.Errorf("runner %s is incompatible with builder %s", runner, builder)
	}

	// TODO generate the run id with a mononotically increasing counter; persist
	//  the run ID in the state db.
	//
	// This Run ID is shared by all groups in the composition.
	runid := uuid.New().String()[24:]

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

	// Coalesce all configurations and deserialise into the config type
	// mandated by the runner.
	obj, err := cfg.CoalesceIntoType(run.ConfigType())
	if err != nil {
		return nil, fmt.Errorf("error while coalescing configuration values: %w", err)
	}

	in := api.RunInput{
		RunID:          runid,
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
		ow.Infow("run finished successfully", "run_id", runid, "plan", plan, "case", tcase, "runner", runner, "instances", in.TotalInstances)
	} else if errors.Is(err, context.Canceled) {
		ow.Infow("run canceled", "run_id", runid, "plan", plan, "case", tcase, "runner", runner, "instances", in.TotalInstances)
	} else {
		ow.Warnw("run finished in error", "run_id", runid, "plan", plan, "case", tcase, "runner", runner, "instances", in.TotalInstances, "error", err)
	}

	return out, err
}

func (e *Engine) DoCollectOutputs(ctx context.Context, runner string, runID string, ow *rpc.OutputWriter) error {
	run, ok := e.runners[runner]
	if !ok {
		return fmt.Errorf("unknown runner: %s", runner)
	}

	var cfg config.CoalescedConfig

	// Get the env config for the runner.
	cfg = cfg.Append(e.envcfg.Runners[runner])

	// Coalesce all configurations and deserialise into the config type
	// mandated by the builder.
	obj, err := cfg.CoalesceIntoType(run.ConfigType())
	if err != nil {
		return fmt.Errorf("error while coalescing configuration values: %w", err)
	}

	input := &api.CollectionInput{
		RunnerID:     runner,
		RunID:        runID,
		EnvConfig:    *e.envcfg,
		RunnerConfig: obj,
	}

	return run.CollectOutputs(ctx, input, ow)
}

func (e *Engine) DoTerminate(ctx context.Context, runner string, ow *rpc.OutputWriter) error {
	run, ok := e.runners[runner]
	if !ok {
		return fmt.Errorf("unknown runner: %s", runner)
	}

	terminatable, ok := run.(api.Terminatable)
	if !ok {
		return fmt.Errorf("runner %s is not terminatable", runner)
	}

	ow.Infof("terminating all jobs on runner: %s", runner)

	err := terminatable.TerminateAll(ctx, ow)
	if err != nil {
		return err
	}

	ow.Infof("all jobs terminated on runner: %s", runner)
	return nil
}

func (e *Engine) DoHealthcheck(ctx context.Context, runner string, fix bool, ow *rpc.OutputWriter) (*api.HealthcheckReport, error) {
	run, ok := e.runners[runner]
	if !ok {
		return nil, fmt.Errorf("unknown runner: %s", runner)
	}

	hc, ok := run.(api.Healthchecker)
	if !ok {
		return nil, fmt.Errorf("runner %s does not support healthchecks", runner)
	}

	ow.Infof("checking runner: %s", runner)

	return hc.Healthcheck(ctx, e, ow, fix)
}

// EnvConfig returns the EnvConfig for this Engine.
func (e *Engine) EnvConfig() config.EnvConfig {
	return *e.envcfg
}

func (e *Engine) Context() context.Context {
	return e.ctx
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
