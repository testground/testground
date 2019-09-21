package runner

import (
	"github.com/ipfs/testground/pkg/api"
)

// Runner is the interface to be implemented by all runners.
type Runner interface {
	// Run runs a test case.
	Run(job *Input, cfg interface{}) (*Output, error)
}

// Input encapsulates the input options for running a test plan.
type Input struct {
	TestPlan *api.TestPlanDefinition
	// Instances is the number of instances to run.
	Instances int
	// ArtifactPath can be a docker image ID or an executable path; it's
	// runner-dependent.
	ArtifactPath string
	// Seq is the test case seq number to run.
	Seq int
	// RunParameters are the runtime parameters to the test case.
	RunParameters map[string]string
}

type Output struct {
	// TODO.
}
