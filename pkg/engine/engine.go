package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/davecgh/go-spew/spew"
	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/build/golang"
	"github.com/ipfs/testground/pkg/config"
	"github.com/ipfs/testground/pkg/rpc"
	"github.com/ipfs/testground/pkg/runner"

	"github.com/google/uuid"
	"github.com/logrusorgru/aurora"
	"golang.org/x/sync/errgroup"
)

// AllBuilders enumerates all builders known to the system.
var AllBuilders = []api.Builder{
	&golang.DockerGoBuilder{},
	&golang.ExecGoBuilder{},
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
	// census is a catalogue of all test plans known to this engine.
	census *TestCensus
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
		census:   newTestCensus(),
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

	if _, err := e.discoverTestPlans(); err != nil {
		return nil, err
	}

	return e, nil
}

func NewDefaultEngine() (*Engine, error) {
	envcfg, err := config.GetEnvConfig()
	if err != nil {
		return nil, err
	}

	cfg := &EngineConfig{
		Builders:  AllBuilders,
		Runners:   AllRunners,
		EnvConfig: envcfg,
	}

	e, err := NewEngine(cfg)
	if err != nil {
		return nil, err
	}

	return e, nil
}

func (e *Engine) TestCensus() api.TestCensus {
	e.lk.RLock()
	defer e.lk.RUnlock()

	return e.census
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

func (e *Engine) DoBuild(ctx context.Context, comp *api.Composition, ow *rpc.OutputWriter) ([]*api.BuildOutput, error) {
	if err := comp.ValidateForBuild(); err != nil {
		return nil, fmt.Errorf("invalid composition: %w", err)
	}

	var (
		testplan = comp.Global.Plan
		builder  = comp.Global.Builder
		plan     = comp.Definition
	)

	//plan := e.TestCensus().PlanByName(testplan)
	//if plan == nil {
	//return nil, fmt.Errorf("unknown test plan: %s", testplan)
	//}

	//if builder == "" {
	//// TODO remove plan-specified runners and builders. Now that we have
	//// compositions, everything must be explicit.
	//builder = plan.Defaults.Builder
	//}

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
		}

		ow.Infof(aurora.Bold(aurora.Green("healthcheck: ok")).String())
	}

	// This var compiles all configurations to coalesce.
	//
	// Precedence (highest to lowest):
	//
	//  1. CLI --run-param, --build-param flags.
	//  2. .env.toml.
	//  3. Test plan definition.
	//  4. Builder defaults (applied by the builder itself, nothing to do here).
	//
	var cfg config.CoalescedConfig

	spew.Dump(plan)

	// 3. Add the base configuration of the build strategy.
	if c, ok := plan.BuildStrategies[builder]; !ok {
		return nil, fmt.Errorf("test plan does not support builder: %s", builder)
	} else {
		cfg = cfg.Append(c)
	}

	// 2. Get the env config for the builder.
	cfg = cfg.Append(e.envcfg.BuildStrategies[builder])

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

			ow.Infow("performing build for groups", "plan", testplan, "groups", grpids, "builder", builder)

			in := &api.BuildInput{
				BuildID:      uuid.New().String()[24:],
				BuildConfig:  obj,
				EnvConfig:    *e.envcfg,
				Directories:  e.envcfg,
				TestPlan:     &plan,
				Selectors:    grp.Build.Selectors,
				Dependencies: grp.Build.Dependencies.AsMap(),
			}

			res, err := bm.Build(ctx, in, ow)
			if err != nil {
				ow.Infow("build failed", "plan", testplan, "groups", grpids, "builder", builder, "error", err)
				return err
			}

			res.BuilderID = bm.ID()

			// no need for a mutex as the indices we access do not intersect
			// across goroutines.
			for _, idx := range uniq[key] {
				ress[idx] = res
			}

			ow.Infow("build succeeded", "plan", testplan, "groups", grpids, "builder", builder, "artifact", res.ArtifactPath)
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
		testplan = comp.Global.Plan
		testcase = comp.Global.Case
		builder  = comp.Global.Builder
		runner   = comp.Global.Runner
		plan     = comp.Definition
	)

	// Find the test plan.
	//plan := e.TestCensus().PlanByName(testplan)
	//if plan == nil {
	//return nil, fmt.Errorf("unrecognized test plan: %s", testplan)
	//}

	// Find the test case.
	seq, tcase, ok := plan.TestCaseByName(testcase)
	if !ok {
		return nil, fmt.Errorf("unrecognized test case %s in test plan %s", testcase, testplan)
	}

	//if runner == "" {
	//// TODO remove plan-specified runners and builders. Now that we have
	//// compositions, everything must be explicit.
	//runner = plan.Defaults.Runner
	//}

	//if builder == "" {
	//// TODO remove plan-specified runners and builders. Now that we have
	//// compositions, everything must be explicit.
	//runner = plan.Defaults.Builder
	//}

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
		}

		ow.Infof(aurora.Bold(aurora.Green("healthcheck: ok")).String())
	}

	// Check if builder and runner are compatible
	if !stringInSlice(comp.Global.Builder, run.CompatibleBuilders()) {
		return nil, fmt.Errorf("runner %s is incompatible with builder %s", runner, builder)
	}

	// Validate the desired number of instances is within bounds.
	if t := int(comp.Global.TotalInstances); t < tcase.Instances.Minimum || t > tcase.Instances.Maximum {
		str := "total instance count outside (%d) of allowable range [%d, %d] for test case %s"
		err := fmt.Errorf(str, t, tcase.Instances.Minimum, tcase.Instances.Maximum, testcase)
		return nil, err
	}

	// TODO generate the run id with a mononotically increasing counter; persist
	// the run ID in the state db.
	//
	// This Run ID is shared by all groups in the composition.
	runid := uuid.New().String()[24:]

	// This var compiles all configurations to coalesce.
	//
	// Precedence (highest to lowest):
	//
	//  1. CLI --run-param, --build-param flags.
	//  2. .env.toml.
	//  3. Test plan definition.
	//  4. Builder defaults (applied by the builder itself, nothing to do here).
	//
	var cfg config.CoalescedConfig

	// Add the base configuration of the run strategy (point 3 above).
	if c, ok := plan.RunStrategies[runner]; !ok {
		return nil, fmt.Errorf("test plan does not support builder: %s", builder)
	} else {
		cfg = cfg.Append(c)
	}

	// 2. Get the env config for the runner.
	cfg = cfg.Append(e.envcfg.RunStrategies[runner])

	// 1. Get overrides from the CLI.
	cfg = cfg.Append(comp.Global.RunConfig)

	// Coalesce all configurations and deserialise into the config type
	// mandated by the runner.
	obj, err := cfg.CoalesceIntoType(run.ConfigType())
	if err != nil {
		return nil, fmt.Errorf("error while coalescing configuration values: %w", err)
	}

	// Create a coalesced configuration for test case parameters.
	defaultParams := make(map[string]string, len(tcase.Parameters))
	for n, v := range tcase.Parameters {
		data, err := json.Marshal(v.Default)
		if err != nil {
			ow.Warnf("failed to parse test case parameter; ignoring; name=%s, value=%v, err=%s", n, v, err)
			continue
		}
		defaultParams[n] = string(data)
	}

	in := api.RunInput{
		RunID:          runid,
		EnvConfig:      *e.envcfg,
		RunnerConfig:   obj,
		Directories:    e.envcfg,
		TestPlan:       &plan,
		Seq:            seq,
		TotalInstances: int(comp.Global.TotalInstances),
		Groups:         make([]api.RunGroup, 0, len(comp.Groups)),
	}

	// Trigger a build for each group, and wait until all of them are done.
	for _, grp := range comp.Groups {
		params := make(map[string]string, len(defaultParams)+len(grp.Run.TestParams))
		for k, v := range defaultParams {
			params[k] = v
		}
		for k, v := range grp.Run.TestParams {
			params[k] = v
		}

		g := api.RunGroup{
			ID:           grp.ID,
			Instances:    int(grp.CalculatedInstanceCount()),
			ArtifactPath: grp.Run.Artifact,
			Parameters:   params,
		}

		in.Groups = append(in.Groups, g)
	}

	out, err := run.Run(ctx, &in, ow)
	if err == nil {
		ow.Infow("run finished successfully", "plan", testplan, "case", testcase, "runner", runner, "instances", in.TotalInstances)
	} else if errors.Is(err, context.Canceled) {
		ow.Infow("run canceled", "plan", testplan, "case", testcase, "runner", runner, "instances", in.TotalInstances)
	} else {
		ow.Warnw("run finished in error", "plan", testplan, "case", testcase, "runner", runner, "instances", in.TotalInstances, "error", err)
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
	cfg = cfg.Append(e.envcfg.RunStrategies[runner])

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
