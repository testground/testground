package api

import (
	"context"

	"github.com/ipfs/testground/pkg/config"
	"github.com/ipfs/testground/pkg/rpc"
)

type Engine interface {
	TestCensus() TestCensus

	BuilderByName(name string) (Builder, bool)
	RunnerByName(name string) (Runner, bool)

	ListBuilders() map[string]Builder
	ListRunners() map[string]Runner

	DoBuild(context.Context, *Composition, *rpc.OutputWriter) ([]*BuildOutput, error)
	DoRun(context.Context, *Composition, *rpc.OutputWriter) (*RunOutput, error)
	DoCollectOutputs(ctx context.Context, comp *Composition, runner string, runID string, ow *rpc.OutputWriter) error
	DoTerminate(ctx context.Context, runner string, ow *rpc.OutputWriter) error
	DoHealthcheck(ctx context.Context, runner string, fix bool, ow *rpc.OutputWriter) (*HealthcheckReport, error)

	EnvConfig() config.EnvConfig
	Context() context.Context
}

type TestCensus interface {
	EnrollTestPlan(tp *TestPlanDefinition) error
	PlanByName(name string) *TestPlanDefinition
	ListPlans() (tp []*TestPlanDefinition)
}
