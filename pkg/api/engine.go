package api

import (
	"context"

	"github.com/ipfs/testground/pkg/config"
	"github.com/ipfs/testground/pkg/tgwriter"
)

type Engine interface {
	TestCensus() TestCensus

	BuilderByName(name string) (Builder, bool)
	RunnerByName(name string) (Runner, bool)

	ListBuilders() map[string]Builder
	ListRunners() map[string]Runner

	DoBuild(context.Context, *Composition, *tgwriter.TgWriter) ([]*BuildOutput, error)
	DoRun(context.Context, *Composition, *tgwriter.TgWriter) (*RunOutput, error)
	DoCollectOutputs(ctx context.Context, runner string, runID string, w *tgwriter.TgWriter) error
	DoTerminate(ctx context.Context, runner string, w *tgwriter.TgWriter) error
	DoHealthcheck(ctx context.Context, runner string, fix bool, w *tgwriter.TgWriter) (*HealthcheckReport, error)

	EnvConfig() config.EnvConfig
	Context() context.Context
}

type TestCensus interface {
	EnrollTestPlan(tp *TestPlanDefinition) error
	PlanByName(name string) *TestPlanDefinition
	ListPlans() (tp []*TestPlanDefinition)
}
