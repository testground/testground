package api

import (
	"context"

	"github.com/testground/testground/pkg/config"
	"github.com/testground/testground/pkg/rpc"
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
	BaseDir string

	// PlanDir is the directory where the test plan's source has been
	// placed (i.e. BaseSrcPath/plan).
	PlanDir string

	// SDKDir is the directory where the SDK's source has been placed. It
	// will be a zero-value if no SDK replacement has been requested, or
	// BaseSrcPath/sdk otherwise.
	SDKDir string

	// ExtraDir is the directory where any extra sources have been unpacked.
	ExtraDir string
}

type Engine interface {
	BuilderByName(name string) (Builder, bool)
	RunnerByName(name string) (Runner, bool)

	ListBuilders() map[string]Builder
	ListRunners() map[string]Runner

	DoBuild(context.Context, *Composition, *UnpackedSources, *rpc.OutputWriter) ([]*BuildOutput, error)
	DoBuildPurge(ctx context.Context, builder, plan string, ow *rpc.OutputWriter) error
	DoRun(context.Context, *Composition, *rpc.OutputWriter) (*RunOutput, error)
	DoCollectOutputs(ctx context.Context, runner string, runID string, ow *rpc.OutputWriter) error
	DoTerminate(ctx context.Context, ctype ComponentType, ref string, ow *rpc.OutputWriter) error
	DoHealthcheck(ctx context.Context, runner string, fix bool, ow *rpc.OutputWriter) (*HealthcheckReport, error)

	EnvConfig() config.EnvConfig
	Context() context.Context
}
