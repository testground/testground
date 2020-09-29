package api

import (
	"context"
	"io"
	"time"

	"github.com/testground/testground/pkg/config"
	"github.com/testground/testground/pkg/rpc"
	"github.com/testground/testground/pkg/task"
)

type ComponentType string

const (
	RunnerType  = ComponentType("runner")
	BuilderType = ComponentType("builder")
)

// UnpackedSources represents the set of directories where a build job unpacks
// its sources.
type UnpackedSources struct {
	// BaseDir is the directory containing the plan under ./plan, and an
	// optional sdk under ./sdk.
	BaseDir string `json:"base_dir"`

	// PlanDir is the directory where the test plan's source has been
	// placed (i.e. BaseSrcPath/plan).
	PlanDir string `json:"plan_dir"`

	// SDKDir is the directory where the SDK's source has been placed. It
	// will be a zero-value if no SDK replacement has been requested, or
	// BaseSrcPath/sdk otherwise.
	SDKDir string `json:"sdk_dir"`

	// ExtraDir is the directory where any extra sources have been unpacked.
	ExtraDir string `json:"extra_dir"`
}

type TasksFilters struct {
	Types    []task.Type
	States   []task.State
	After    *time.Time
	Before   *time.Time
	TestPlan string
	TestCase string
}

type Engine interface {
	BuilderByName(name string) (Builder, bool)
	RunnerByName(name string) (Runner, bool)

	ListBuilders() map[string]Builder
	ListRunners() map[string]Runner

	QueueBuild(request *BuildRequest, sources *UnpackedSources) (string, error)
	QueueRun(request *RunRequest, sources *UnpackedSources) (string, error)

	Logs(ctx context.Context, id string, follow bool, cancel bool, w io.Writer) (*task.Task, error)

	Status(id string) (*task.Task, error)
	Tasks(filters TasksFilters) ([]task.Task, error)

	DoBuildPurge(ctx context.Context, builder, plan string, ow *rpc.OutputWriter) error
	DoCollectOutputs(ctx context.Context, runner string, runID string, ow *rpc.OutputWriter) error
	DoTerminate(ctx context.Context, ctype ComponentType, ref string, ow *rpc.OutputWriter) error
	DoHealthcheck(ctx context.Context, runner string, fix bool, ow *rpc.OutputWriter) (*HealthcheckReport, error)

	EnvConfig() config.EnvConfig
	Context() context.Context
}
