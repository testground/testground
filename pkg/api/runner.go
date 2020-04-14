package api

import (
	"context"
	"reflect"

	"github.com/ipfs/testground/pkg/config"
	"github.com/ipfs/testground/pkg/rpc"
)

// Runner is the interface to be implemented by all runners. A runner takes a
// test plan in executable form and schedules a run of a particular test case
// within it.
//
// TODO cardinality: do we want to be able to run multiple test cases within a
// test plan in a single call?
type Runner interface {
	// ID returns the canonical identifier for this runner.
	ID() string

	// Run runs a test case.
	Run(ctx context.Context, job *RunInput, ow *rpc.OutputWriter) (*RunOutput, error)

	// ConfigType returns the configuration type of this runner.
	ConfigType() reflect.Type

	// CompatibleBuilders returns the IDs of the builders whose artifacts this
	// runner can work with.
	CompatibleBuilders() []string

	// CollectOutputs gathers the outputs from a run, and produces a zip file
	// with the contents, writing it to the specified io.Writer.
	CollectOutputs(context.Context, *CollectionInput, *rpc.OutputWriter) error
}

// RunInput encapsulates the input options for running a test plan.
type RunInput struct {
	// RunID is the run id assigned to this job by the Engine.
	RunID string

	// EnvConfig is the env configuration of the engine. Not a pointer to force
	// a copy.
	EnvConfig config.EnvConfig

	// RunnerConfig is the configuration of the runner sourced from the test
	// plan manifest, coalesced with any user-provided overrides.
	RunnerConfig interface{}

	// Directories providers accessors to directories managed by the runtime.
	Directories Directories

	// TestPlan is the definition of the test plan containing the test case to
	// run.
	TestPlan *TestPlanDefinition

	// TestCase is the the definition of the test case to run.
	TestCase *TestCase

	// TotalInstances is the total number of instances participating in this test case.
	TotalInstances int

	// Groups enumerates the groups participating in this run.
	Groups []RunGroup
}

type RunGroup struct {
	// ID is the id of the instance group this run pertains to.
	ID string

	// Instances is the number of instances to run with this configuration.
	Instances int

	// Resources for per instance in this group
	Resources Resources

	// ArtifactPath can be a docker image ID or an executable path; it's
	// runner-dependent.
	ArtifactPath string

	// Parameters are the runtime parameters to the test case.
	Parameters map[string]string
}

type RunOutput struct {
	// RunnerID is the ID of the runner used.
	RunID string
}

type CollectionInput struct {
	// EnvConfig is the env configuration of the engine. Not a pointer to force
	// a copy.
	EnvConfig config.EnvConfig
	RunID     string
	RunnerID  string

	// RunnerConfig is the configuration of the runner sourced from the test
	// plan manifest, coalesced with any user-provided overrides.
	RunnerConfig interface{}
}

// Terminatable is the interface to be implemented by a runner that can be
// terminated.
type Terminatable interface {
	TerminateAll(context.Context, *rpc.OutputWriter) error
}
