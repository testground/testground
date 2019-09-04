package api

import (
	napi "github.com/hashicorp/nomad/api"
)

type BuildOpts struct {
	WorkDir  string
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

// TestCaseDefinition encapsulates the definition of a test case.
type TestCaseDefinition struct {
	// Name is the name of the test case.
	Name string
	// Instances is the amount of instances this test case requires.
	Instances int
}

type TestCase interface {
	// Define returns the definition of a test case.
	Define() *TestCaseDefinition

	//
	Mutate(*napi.Job)
}

type TestPlanDefinition struct {
	Name      string
	TestCases []TestCase
}

type Namer interface {
	Name() string
}

// TestPlan is to be implemented by all test plans.
type TestPlan interface {
	// Define retuns the definition of this test plan.
	Define() *TestPlanDefinition

	// Build takes the source of the test plan and compiles it into an
	// executable or Docker image that can then be shipped somewhere for
	// execution.
	Build(*BuildOpts) (*BuildResult, error)

	// Schedule emits the Nomad jobs to schedule the test cases within this
	// test plan.
	Schedule(*BuildResult, *ScheduleOpts) (*ScheduleResult, error)
}
