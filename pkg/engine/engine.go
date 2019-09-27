package engine

import (
	"bytes"
	"fmt"
	"reflect"
	"sync"

	"github.com/google/uuid"

	"github.com/BurntSushi/toml"

	"github.com/ipfs/testground/pkg/build"
	"github.com/ipfs/testground/pkg/runner"
)

// AllBuilders enumerates all builders known to the system.
var AllBuilders = []build.Builder{
	&build.DockerGoBuilder{},
}

// AllRunners enumerates all builders known to the system.
var AllRunners = []runner.Runner{
	&runner.LocalDockerRunner{},
	&runner.LocalExecutableRunner{},
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
	builders map[string]build.Builder
	// runners binds runners to their identifying key.
	runners map[string]runner.Runner
}

type EngineConfig struct {
	Builders []build.Builder
	Runners  []runner.Runner
}

func NewEngine(cfg *EngineConfig) *Engine {
	e := &Engine{
		census:   newTestCensus(),
		builders: make(map[string]build.Builder, len(cfg.Builders)),
		runners:  make(map[string]runner.Runner, len(cfg.Runners)),
	}

	for _, b := range cfg.Builders {
		e.builders[b.ID()] = b
	}

	for _, r := range cfg.Runners {
		e.runners[r.ID()] = r
	}

	return e
}

func NewDefaultEngine() *Engine {
	cfg := &EngineConfig{
		Builders: AllBuilders,
		Runners:  AllRunners,
	}

	engine := NewEngine(cfg)

	for _, p := range discoverTestPlans() {
		engine.census.EnrollTestPlan(p)
	}

	return engine
}

func (e *Engine) TestCensus() *TestCensus {
	e.lk.RLock()
	defer e.lk.RUnlock()

	return e.census
}

func (e *Engine) BuilderByName(name string) (build.Builder, bool) {
	e.lk.RLock()
	defer e.lk.RUnlock()

	m, ok := e.builders[name]
	return m, ok
}

func (e *Engine) RunnerByName(name string) (runner.Runner, bool) {
	e.lk.RLock()
	defer e.lk.RUnlock()

	m, ok := e.runners[name]
	return m, ok
}

func (e *Engine) ListBuilders() map[string]build.Builder {
	e.lk.RLock()
	defer e.lk.RUnlock()

	m := make(map[string]build.Builder, len(e.builders))
	for k, v := range e.builders {
		m[k] = v
	}
	return m
}

func (e *Engine) ListRunners() map[string]runner.Runner {
	e.lk.RLock()
	defer e.lk.RUnlock()

	m := make(map[string]runner.Runner, len(e.runners))
	for k, v := range e.runners {
		m[k] = v
	}
	return m
}

func (e *Engine) DoBuild(testplan string, builder string, input *build.Input) (*build.Output, error) {
	plan := e.TestCensus().PlanByName(testplan)
	if plan == nil {
		return nil, fmt.Errorf("unknown test plan: %s", testplan)
	}

	// Find the builder.
	bm, ok := e.builders[builder]
	if !ok {
		return nil, fmt.Errorf("unrecognized builder: %s", builder)
	}

	// Get the base configuration of the build strategy.
	planCfg, ok := plan.BuildStrategies[builder]
	if !ok {
		return nil, fmt.Errorf("test plan does not support builder: %s", builder)
	}

	// Compile all configurations to coalesce.
	cfgs := []map[string]interface{}{planCfg}
	if overrides := input.BuildConfig; overrides != nil {
		// We have config overrides.
		// TODO type conversion.
		cfgs = append(cfgs, overrides.(map[string]interface{}))
	}

	// Coalesce all configs and deserialize into the provided type.
	cfg, err := coalesceConfigsIntoType(bm.ConfigType(), cfgs...)
	if err != nil {
		return nil, fmt.Errorf("error while coalescing configuration values: %w", err)
	}

	input.TestPlan = plan
	input.BaseDir = BaseDir
	input.BuildConfig = cfg

	return bm.Build(input)
}

func (e *Engine) DoRun(testplan string, testcase string, runner string, input *runner.Input) (*runner.Output, error) {
	// Find the test plan.
	plan := e.TestCensus().PlanByName(testplan)
	if plan == nil {
		return nil, fmt.Errorf("unrecognized test plan: %s", testplan)
	}

	// Find the test case.
	seq, tcase, ok := plan.TestCaseByName(testcase)
	if !ok {
		return nil, fmt.Errorf("unrecognized test case %s in test plan %s", testcase, testplan)
	}

	// Get the runner.
	run, ok := e.runners[runner]
	if !ok {
		return nil, fmt.Errorf("unknown runner: %s", runner)
	}

	// Fall back to default instance count if none was provided.
	if input.Instances == 0 {
		input.Instances = tcase.Instances.Default
	}

	// Get the base configuration of the run strategy.
	planCfg, ok := plan.RunStrategies[runner]
	if !ok {
		return nil, fmt.Errorf("test plan does not support runner: %s", runner)
	}

	// Compile all configurations to coalesce.
	cfgs := []map[string]interface{}{planCfg}
	if overrides := input.RunnerConfig; overrides != nil {
		// We have config overrides.
		// TODO type conversions.
		cfgs = append(cfgs, overrides.(map[string]interface{}))
	}

	// Coalesce all configs and deserialize into the provided type.
	cfg, err := coalesceConfigsIntoType(run.ConfigType(), cfgs...)
	if err != nil {
		return nil, fmt.Errorf("error while coalescing configuration values: %w", err)
	}

	// TODO generate the run id with a mononotically increasing counter; persist
	// the run ID in the state db.
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("error while generating test run ID: %w", err)
	}

	input.ID = id.String()
	input.RunnerConfig = cfg
	input.Seq = seq
	input.TestPlan = plan

	return run.Run(input)
}

func coalesceConfigsIntoType(typ reflect.Type, cfgs ...map[string]interface{}) (interface{}, error) {
	m := make(map[string]interface{})

	// Copy all values into coalesced map.
	for _, cfg := range cfgs {
		for k, v := range cfg {
			m[k] = v
		}
	}

	// Serialize map into TOML, and then deserialize into the appropriate type.
	buf := new(bytes.Buffer)
	if err := toml.NewEncoder(buf).Encode(m); err != nil {
		return nil, fmt.Errorf("error while encoding into TOML: %w", err)
	}

	v := reflect.New(typ).Interface()
	_, err := toml.DecodeReader(buf, v)
	return v, err
}
