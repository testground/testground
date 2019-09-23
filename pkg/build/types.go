package build

import (
	"github.com/ipfs/testground/pkg/api"
)

// Builder is the interface to be implemented by all builders.
type Builder interface {
	// Build performs a build. It takes the definition of the build job, and the
	// configuration of this builder as per the test plan manifest.
	Build(job *Input) (*Output, error)

	// OverridableParameters returns the names of the build configuration
	// parameters than can be overriden.
	OverridableParameters() []string
}

// Input encapsulates the input options for building a test plan.
type Input struct {
	// TestPlan is the metadata of the test plan being built.
	TestPlan *api.TestPlanDefinition
	// BaseDir is the base directory of the testground source code.
	BaseDir string
	// Dependencies are the versions of upstream dependencies we want to build
	// against. For a go build, this could be e.g.:
	//  github.com/ipfs/go-ipfs=v0.4.22
	//  github.com/libp2p/go-libp2p=v0.2.8
	Dependencies map[string]string
	// BuildConfig is the configuration of the build job sourced from the test
	// plan manifest.
	BuildConfig interface{}
	// BuildConfigOverride are override parameters passed in when the build job
	// was triggered.
	BuildConfigOverride map[string]string
}

// Output encapsulates the output from a build action.
type Output struct {
	// ArtifactPath can be the docker image ID, a file location, etc. of the
	// resulting artifact. It is builder-dependent.
	ArtifactPath string
}
