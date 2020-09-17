package engine

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/xid"
	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/build"
	"github.com/testground/testground/pkg/config"
	"github.com/testground/testground/pkg/rpc"
	"github.com/testground/testground/pkg/runner"
	"github.com/testground/testground/pkg/task"
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
// queryable, e.g. what tests are running, or have run, such that we can easily
// query test plans that ran for a particular commit of an upstream.
type Engine struct {
	lk sync.RWMutex
	// builders binds builders to their identifying key.
	builders map[string]api.Builder
	// runners binds runners to their identifying key.
	runners map[string]api.Runner
	envcfg  *config.EnvConfig
	ctx     context.Context
	store   *task.Storage
	queue   *task.Queue
	// signals contains a channel for each running task
	// by closing a channel, the task is canceled
	signals   map[string]chan int
	signalsLk sync.RWMutex
}

var _ api.Engine = (*Engine)(nil)

type EngineConfig struct {
	Builders  []api.Builder
	Runners   []api.Runner
	EnvConfig *config.EnvConfig
}

func NewEngine(cfg *EngineConfig) (*Engine, error) {
	var (
		store *task.Storage
		err   error
	)

	if cfg.EnvConfig.Daemon.TasksInMemory {
		store, err = task.NewMemoryTaskStorage()
	} else {
		path := filepath.Join(cfg.EnvConfig.Dirs().Home(), "tasks.db")
		store, err = task.NewTaskStorage(path)
	}

	if err != nil {
		return nil, err
	}

	queue, err := task.NewQueue(store, cfg.EnvConfig.Daemon.QueueSize)
	if err != nil {
		return nil, err
	}

	e := &Engine{
		builders: make(map[string]api.Builder, len(cfg.Builders)),
		runners:  make(map[string]api.Runner, len(cfg.Runners)),
		envcfg:   cfg.EnvConfig,
		ctx:      context.Background(),
		store:    store,
		queue:    queue,
		signals:  make(map[string]chan int),
	}

	for _, b := range cfg.Builders {
		e.builders[b.ID()] = b
	}

	for _, r := range cfg.Runners {
		e.runners[r.ID()] = r
	}

	for i := 0; i < cfg.EnvConfig.Daemon.Workers; i++ {
		go e.worker(i)
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

func (e *Engine) QueueBuild(request *api.BuildRequest, sources *api.UnpackedSources) (string, error) {
	id := xid.New().String()
	err := e.queue.Push(&task.Task{
		Version:  0,
		Priority: request.Priority,
		ID:       id,
		Type:     task.TypeBuild,
		Input: &BuildInput{
			BuildRequest: request,
			Sources:      sources,
		},
		States: []task.DatedState{
			{
				State:   task.StateScheduled,
				Created: time.Now().UTC(),
			},
		},
	})

	return id, err
}

func (e *Engine) QueueRun(request *api.RunRequest, sources *api.UnpackedSources) (string, error) {
	var (
		builder = request.Composition.Global.Builder
		runner  = request.Composition.Global.Runner
	)

	// Get the runner.
	run, ok := e.runners[runner]
	if !ok {
		return "", fmt.Errorf("unknown runner: %s", runner)
	}

	// Check if builder and runner are compatible
	if !stringInSlice(builder, run.CompatibleBuilders()) {
		return "", fmt.Errorf("runner %s is incompatible with builder %s", runner, builder)
	}

	id := xid.New().String()
	err := e.queue.Push(&task.Task{
		Version:  0,
		Priority: request.Priority,
		Plan:     request.Composition.Global.Plan,
		Case:     request.Composition.Global.Case,
		ID:       id,
		Type:     task.TypeRun,
		Input: &RunInput{
			RunRequest: request,
			Sources:    sources,
		},
		States: []task.DatedState{
			{
				State:   task.StateScheduled,
				Created: time.Now().UTC(),
			},
		},
	})

	return id, err
}

func (e *Engine) DoCollectOutputs(ctx context.Context, runner string, runID string, ow *rpc.OutputWriter) error {
	run, ok := e.runners[runner]
	if !ok {
		return fmt.Errorf("unknown runner: %s", runner)
	}

	var cfg config.CoalescedConfig

	// Get the env config for the runner.
	cfg = cfg.Append(e.envcfg.Runners[runner])

	// Coalesce all configurations and deserialize into the config type
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

func (e *Engine) DoTerminate(ctx context.Context, ctype api.ComponentType, ref string, ow *rpc.OutputWriter) error {
	var component interface{}
	var ok bool
	switch ctype {
	case api.RunnerType:
		component, ok = e.runners[ref]
	case api.BuilderType:
		component, ok = e.builders[ref]
	}

	if !ok {
		return fmt.Errorf("unknown component: %s (type: %s)", ref, ctype)
	}

	terminatable, ok := component.(api.Terminatable)
	if !ok {
		return fmt.Errorf("component %s is not terminatable", ref)
	}

	ow.Infof("terminating all jobs on component: %s", ref)

	err := terminatable.TerminateAll(ctx, ow)
	if err != nil {
		return err
	}

	ow.Infof("all jobs terminated on component: %s", ref)
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

func (e *Engine) DoBuildPurge(ctx context.Context, builder, plan string, ow *rpc.OutputWriter) error {
	bm, ok := e.builders[builder]
	if !ok {
		return fmt.Errorf("unrecognized builder: %s", builder)
	}
	return bm.Purge(ctx, plan, ow)
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

func (e *Engine) Tasks(filters api.TasksFilters) ([]task.Task, error) {
	var res []task.Task

	before := time.Now().UTC().Add(-24 * time.Hour)
	after := time.Now().UTC()

	e.signalsLk.RLock()

	for _, state := range filters.States {
		var prefix string

		switch state {
		case task.StateScheduled:
			prefix = task.QUEUEPREFIX
		case task.StateProcessing:
			prefix = task.CURRENTPREFIX
		case task.StateComplete:
			prefix = task.ARCHIVEPREFIX
		}

		tsks, err := e.store.Range(prefix, before, after)
		if err != nil {
			return nil, err
		}

		for _, tsk := range tsks {
			for _, tp := range filters.Types {
				if tsk.Type == tp {
					res = append(res, *tsk)
					break
				}
			}
		}
	}

	e.signalsLk.RUnlock()
	return res, nil
}

func (e *Engine) Status(id string) (*task.Task, error) {
	tsk, err := e.store.Get(task.ARCHIVEPREFIX, id)
	if err == nil {
		return tsk, nil
	}
	if err != task.ErrNotFound {
		return nil, err
	}
	tsk, err = e.store.Get(task.CURRENTPREFIX, id)
	if err == nil {
		return tsk, nil
	}
	if err != task.ErrNotFound {
		return nil, err
	}
	return e.store.Get(task.QUEUEPREFIX, id)
}

func (e *Engine) Logs(ctx context.Context, id string, follow bool, cancel bool, ow *rpc.OutputWriter) (*task.Task, error) {
	path := filepath.Join(e.EnvConfig().Dirs().Daemon(), id+".out")

	if !follow {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)

		for scanner.Scan() {
			_, err = ow.WriteProgress(scanner.Bytes())
			if err != nil {
				return nil, err
			}

			if err := scanner.Err(); err != nil {
				return nil, err
			}
		}

		return e.Status(id)
	}

	// Wait for the task to start
	for {
		tsk, err := e.Status(id)
		if err != nil {
			return nil, err
		}

		if tsk.State().State == task.StateScheduled {
			time.Sleep(time.Millisecond * 500)
		} else {
			break
		}
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := bufio.NewReader(file)

	var prevBytes []byte

Outer:
	for {
		select {
		case <-ctx.Done():
			if cancel {
				e.signalsLk.RLock()
				if ch, ok := e.signals[id]; ok {
					close(ch)
				}
				e.signalsLk.RUnlock()
			}
			break Outer
		default:
			e.signalsLk.RLock()
			_, running := e.signals[id]
			e.signalsLk.RUnlock()

			line, err := reader.ReadBytes('\n')

			if err == io.EOF {
				if len(line) != 0 {
					// It means we read part of a line so it's not actually
					// the end of the file.
					prevBytes = line
					continue
				}

				if running {
					continue
				} else {
					break Outer
				}
			} else if err != nil {
				return nil, err
			}

			if prevBytes != nil {
				line = append(prevBytes, line...)
			}

			prevBytes = nil
			_, err = ow.WriteProgress(line)
			if err != nil {
				return nil, err
			}
		}
	}

	return e.Status(id)
}
