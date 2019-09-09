package engine

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/ipfs/testground/pkg/api"

	"github.com/ipfs/testground/pkg/build"
	"github.com/ipfs/testground/pkg/runner"
)

type BuilderMapping struct {
	Key            string
	Builder        build.Builder
	ConfigType     reflect.Type
	configFieldIdx int
}

type RunnerMapping struct {
	Key            string
	Runner         runner.Runner
	ConfigType     reflect.Type
	configFieldIdx int
}

var AllBuilders = []BuilderMapping{
	{
		Key:        "docker:go",
		Builder:    &build.DockerGoBuilder{},
		ConfigType: reflect.TypeOf(&api.GoBuildStrategy{}),
	},
}

var AllRunners = []RunnerMapping{
	{
		Key:        "local:exec",
		Runner:     &runner.LocalExecutableRunner{},
		ConfigType: reflect.TypeOf(&api.PlaceholderRunStrategy{}),
	},
}

type Engine struct {
	lk       sync.RWMutex
	census   *TestCensus
	builders map[string]BuilderMapping
	runners  map[string]RunnerMapping
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
	cfg := f.Elem().Interface()
	return bm.Builder.Build(input, cfg)
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
