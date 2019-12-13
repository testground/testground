package engine

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"sync"

	"github.com/google/uuid"

	"github.com/BurntSushi/toml"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/build/golang"
	"github.com/ipfs/testground/pkg/config"
	"github.com/ipfs/testground/pkg/runner"
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
}

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
	}

	for _, b := range cfg.Builders {
		e.builders[b.ID()] = b
	}

	for _, r := range cfg.Runners {
		e.runners[r.ID()] = r
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

	_, _ = e.discoverTestPlans()

	return e, nil
}

func (e *Engine) TestCensus() *TestCensus {
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

func (e *Engine) DoBuild(testplan string, builder string, input *api.BuildInput, output io.Writer) (*api.BuildOutput, error) {
	plan := e.TestCensus().PlanByName(testplan)
	if plan == nil {
		return nil, fmt.Errorf("unknown test plan: %s", testplan)
	}

	if builder == "" {
		builder = plan.Defaults.Builder
	}

	// Find the builder.
	bm, ok := e.builders[builder]
	if !ok {
		return nil, fmt.Errorf("unrecognized builder: %s", builder)
	}

	// Compile all configurations to coalesce. Precedence (highest to lowest):
	//  1. CLI --run-param, --build-param flags.
	//  2. .env.toml.
	//  3. Test plan definition.
	//  4. Builder defaults (applied by the builder).
	var cfgs []map[string]interface{}

	// 3. Get the base configuration of the build strategy.
	planCfg, ok := plan.BuildStrategies[builder]
	if !ok {
		return nil, fmt.Errorf("test plan does not support builder: %s", builder)
	}
	cfgs = append(cfgs, planCfg)

	// 2. Get the env config for the builder.
	envCfg, ok := e.envcfg.BuildStrategies[builder]
	if ok {
		cfgs = append(cfgs, envCfg)
	}

	// 1. Get overrides from the CLI.
	if overrides := input.BuildConfig; overrides != nil {
		// We have config overrides.
		cfgs = append(cfgs, overrides.(map[string]interface{}))
	}

	// Coalesce all configs and deserialize into the provided type.
	cfg, err := coalesceConfigsIntoType(bm.ConfigType(), cfgs...)
	if err != nil {
		return nil, fmt.Errorf("error while coalescing configuration values: %w", err)
	}

	input.BuildID = uuid.New().String()
	input.Directories = e.envcfg
	input.TestPlan = plan
	input.BuildConfig = cfg
	input.EnvConfig = *e.envcfg

	res, err := bm.Build(input, output)
	if err != nil {
		return nil, err
	}
	res.BuilderID = bm.ID()
	return res, err
}

func (e *Engine) DoRun(testplan string, testcase string, runner string, input *api.RunInput, output io.Writer) (*api.RunOutput, error) {
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

	if runner == "" {
		runner = plan.Defaults.Runner
	}

	// Get the runner.
	run, ok := e.runners[runner]
	if !ok {
		return nil, fmt.Errorf("unknown runner: %s", runner)
	}

	// Check if builder and runner are compatible
	if !stringInSlice(input.BuilderID, run.CompatibleBuilders()) {
		return nil, fmt.Errorf("cannot use runner %v with build from %v", runner, input.BuilderID)
	}

	// Fall back to default instance count if none was provided.
	// Else validate the desired number of instances is within bounds.
	if instances := input.Instances; instances == 0 {
		input.Instances = tcase.Instances.Default
	} else if instances < tcase.Instances.Minimum || instances > tcase.Instances.Maximum {
		str := "instance count outside (%d) of allowable range [%d, %d] for test %s"
		err := fmt.Errorf(str, instances, tcase.Instances.Minimum, tcase.Instances.Maximum, testcase)
		return nil, err
	}

	// Compile all configurations to coalesce. Precedence (highest to lowest):
	//  1. CLI --run-param, --build-param flags.
	//  2. .env.toml.
	//  3. Test plan definition.
	//  4. Runner defaults (applied by the runner).
	var cfgs []map[string]interface{}

	// 3. Get the base configuration of the run strategy.
	planCfg, ok := plan.RunStrategies[runner]
	if !ok {
		return nil, fmt.Errorf("test plan does not support runner: %s", runner)
	}
	cfgs = append(cfgs, planCfg)

	// 2. Get the env config for the runner.
	envCfg, ok := e.envcfg.RunStrategies[runner]
	if ok {
		cfgs = append(cfgs, envCfg)
	}

	// 1. Get overrides from the CLI.
	if overrides := input.RunnerConfig; overrides != nil {
		// We have config overrides.
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

	input.RunID = id.String()
	input.RunnerConfig = cfg
	input.Seq = seq
	input.TestPlan = plan
	input.EnvConfig = *e.envcfg

	return run.Run(input, output)
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

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
