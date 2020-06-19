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

type Engine interface {
	BuilderByName(name string) (Builder, bool)
	RunnerByName(name string) (Runner, bool)

	ListBuilders() map[string]Builder
	ListRunners() map[string]Runner

	DoBuild(context.Context, *Composition, string, string, string, *rpc.OutputWriter) ([]*BuildOutput, error)
	DoRun(context.Context, *Composition, *rpc.OutputWriter) (*RunOutput, error)
	DoCollectOutputs(ctx context.Context, runner string, runID string, ow *rpc.OutputWriter) error
	DoTerminate(ctx context.Context, ctype ComponentType, ref string, ow *rpc.OutputWriter) error
	DoHealthcheck(ctx context.Context, runner string, fix bool, ow *rpc.OutputWriter) (*HealthcheckReport, error)

	EnvConfig() config.EnvConfig
	Context() context.Context
}
