package api

import (
	"context"
	"io"

	"github.com/ipfs/testground/pkg/config"
)

type Engine interface {
	TestCensus() TestCensus

	BuilderByName(name string) (Builder, bool)
	RunnerByName(name string) (Runner, bool)

	ListBuilders() map[string]Builder
	ListRunners() map[string]Runner

	DoBuild(context.Context, *Composition, io.Writer) ([]*BuildOutput, error)
	DoRun(context.Context, *Composition, io.Writer) (*RunOutput, error)
	DoCollectOutputs(ctx context.Context, runner string, runID string, w io.Writer) error
	DoTerminate(ctx context.Context, runner string, w io.Writer) error
	DoHealthcheck(ctx context.Context, runner string, fix bool, w io.Writer) (*HealthcheckReport, error)

	EnvConfig() config.EnvConfig
	Context() context.Context
}

type TestCensus interface {
	EnrollTestPlan(tp *TestPlanDefinition) error
	PlanByName(name string) *TestPlanDefinition
	ListPlans() (tp []*TestPlanDefinition)
}
