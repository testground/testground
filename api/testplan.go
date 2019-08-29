package api

import (
	napi "github.com/hashicorp/nomad/api"
)

type BuildOpts struct {
	Versions map[string]string
}

type BuildResult struct {
	DockerImage string
}

type ScheduleOpts struct {
}

type ScheduleResult struct {
	Jobs []*napi.Job
}

type TestPlanDescriptor struct {
	Name      string
	TestCases []string
}

type Namer interface {
	Name() string
}

type TestPlan interface {
	Descriptor() *TestPlanDescriptor

	// Build takes the source of the test plan and compiles it into an
	// executable or Docker image that can then be shipped somewhere for
	// execution.
	Build(*BuildOpts) (*BuildResult, error)

	// Schedule emits the Nomad jobs to schedule the test cases within this
	// test plan.
	Schedule(*BuildResult, *ScheduleOpts) (*ScheduleResult, error)
}
