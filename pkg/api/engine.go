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

	EnvConfig() config.EnvConfig
	Context() context.Context
}

type TestCensus interface {
	EnrollTestPlan(tp *TestPlanDefinition) error
	PlanByName(name string) *TestPlanDefinition
	ListPlans() (tp []*TestPlanDefinition)
}
