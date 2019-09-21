package build

import (
	"github.com/ipfs/testground/pkg/api"
)

// Builder is the interface to be implemented by all builders.
type Builder interface {
	Build(job *Input, cfg interface{}) (*Output, error)
}

// Input encapsulates the input options for building a test plan.
type Input struct {
	TestPlan        *api.TestPlanDefinition
	BaseDir         string
	Dependencies    map[string]string
	BuildParameters map[string]string
}

// Output encapsulates the output from a build action.
type Output struct {
	ArtifactPath string
}
