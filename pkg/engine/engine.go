package engine

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/build"
	"github.com/ipfs/testground/pkg/runner"
)

// BuilderMapping provides metadata about a builder that has been loaded into
// the system.
type BuilderMapping struct {
	// Key is the identifier by which this builder strategy will be referenced
	// in configuration files.
	Key string
	// Builder is the actual builder.
	Builder build.Builder
	// ConfigType is the type of the configuration object this builder receives.
	// It is used to cherry-pick the configuration from a test plan definition.
	ConfigType reflect.Type
	// configFieldIdx caches the index of the configuration object within the
	// BuilderStrategies configuration entity.
	configFieldIdx int
}

// RunnerMapping provides metadata about a runner that has been loaded into
// the system.
type RunnerMapping struct {
	// Key is the identifier by which this run strategy will be referenced
	// in configuration files.
	Key string
	// Builder is the actual runner.
	Runner runner.Runner
	// ConfigType is the type of the configuration object this runner receives.
	// It is used to cherry-pick the configuration from a test plan definition.
	ConfigType reflect.Type
	// CompatibleBuilders references the builders this runner can be combined
	// with.
	CompatibleBuilders []*BuilderMapping
	// configFieldIdx caches the index of the configuration object within the
	// RunStrategies configuration entity.
	configFieldIdx int
}

// AllBuilders enumerates all builders known to the system.
var AllBuilders = []BuilderMapping{
	{
		Key:        "docker:go",
		Builder:    &build.DockerGoBuilder{},
		ConfigType: reflect.TypeOf(&api.GoBuildStrategy{}),
	},
}

// AllRunners enumerates all builders known to the system.
var AllRunners = []RunnerMapping{
	{
		Key:                "local:docker",
		Runner:             &runner.LocalDockerRunner{},
		CompatibleBuilders: []*BuilderMapping{&AllBuilders[0]},
		ConfigType:         reflect.TypeOf(&api.PlaceholderRunStrategy{}),
	},
	{
		Key:        "local:exec",
		Runner:     &runner.LocalExecutableRunner{},
		ConfigType: reflect.TypeOf(&api.PlaceholderRunStrategy{}),
		// CompatibleBuilders: TODO,
	},
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
	builders map[string]BuilderMapping
	// runners binds runners to their identifying key.
	runners map[string]RunnerMapping
}

type EngineConfig struct {
	Builders []BuilderMapping
	Runners  []RunnerMapping
}

func NewEngine(cfg *EngineConfig) *Engine {
	e := &Engine{
		census:   newTestCensus(),
		builders: make(map[string]BuilderMapping, len(cfg.Builders)),
		runners:  make(map[string]RunnerMapping, len(cfg.Runners)),
	}

	typ := reflect.TypeOf(api.BuildStrategies{})
	for _, b := range cfg.Builders {
		found := false
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			ft := f.Type
			if ft == b.ConfigType {
				b.configFieldIdx = i
				found = true
				break
			}
		}
		if !found {
			panic("could not find config field for builder " + b.Key)
		}
		e.builders[b.Key] = b

	}

	typ = reflect.TypeOf(api.RunStrategies{})
	for _, r := range cfg.Runners {
		found := false
		for i := 0; i < typ.NumField(); i++ {
			if typ.Field(i).Type == r.ConfigType {
				r.configFieldIdx = i
				found = true
				break
			}
		}
		if !found {
			panic("could not find config field for runner " + r.Key)
		}
		e.runners[r.Key] = r
	}

	return e
}

func NewDefaultEngine() *Engine {
	var (
		cfg    = &EngineConfig{AllBuilders, AllRunners}
		engine = NewEngine(cfg)
		plans  = discoverTestPlans()
	)

	for _, p := range plans {
		engine.census.EnrollTestPlan(p)
	}

	return engine
}

func (e *Engine) TestCensus() *TestCensus {
	e.lk.RLock()
	defer e.lk.RUnlock()

	return e.census
}

func (e *Engine) BuilderByName(name string) build.Builder {
	e.lk.RLock()
	defer e.lk.RUnlock()

	if m, ok := e.builders[name]; ok {
		return m.Builder
	} else {
		return nil
	}
}

func (e *Engine) RunnerByName(name string) runner.Runner {
	e.lk.RLock()
	defer e.lk.RUnlock()

	if m, ok := e.runners[name]; ok {
		return m.Runner
	} else {
		return nil
	}
}

func (e *Engine) ListBuilders() map[string]build.Builder {
	e.lk.RLock()
	defer e.lk.RUnlock()

	m := make(map[string]build.Builder, len(e.builders))
	for k, v := range e.builders {
		m[k] = v.Builder
	}
	return m
}

func (e *Engine) ListRunners() map[string]runner.Runner {
	e.lk.RLock()
	defer e.lk.RUnlock()

	m := make(map[string]runner.Runner, len(e.runners))
	for k, v := range e.runners {
		m[k] = v.Runner
	}
	return m
}

func (e *Engine) DoBuild(testplan string, builder string, input *build.Input) (*build.Output, error) {
	tp := e.TestCensus().ByName(testplan)
	if tp == nil {
		return nil, fmt.Errorf("unknown test plan: %s", testplan)
	}
	bm, ok := e.builders[builder]
	if !ok {
		return nil, fmt.Errorf("unknown builder: %s", builder)
	}

	input.TestPlan = tp
	input.BaseDir = BaseDir

	// find the field that matches the toml key for the build strategy.
	f := reflect.ValueOf(tp.BuildStrategies).Field(bm.configFieldIdx)
	if f.IsZero() || f.IsNil() {
		return nil, fmt.Errorf("plan does not support builder %s", builder)
	}

	// field is a pointer, so we get Elem().
	input.BuildConfig = f.Elem().Interface()
	return bm.Builder.Build(input)
}

// TODO does run need to trigger a build?
func (e *Engine) DoRun(testplan string, runner string, input *runner.Input) (*runner.Output, error) {
	tp := e.TestCensus().ByName(testplan)
	if tp == nil {
		return nil, fmt.Errorf("unknown test plan: %s", testplan)
	}
	rm, ok := e.runners[runner]
	if !ok {
		return nil, fmt.Errorf("unknown runner: %s", runner)
	}

	input.TestPlan = tp

	// find the field that matches the toml key for the run strategy.
	cfg := reflect.ValueOf(tp.RunStrategies).Field(rm.configFieldIdx).Elem().Interface()
	return rm.Runner.Run(input, cfg)
}
