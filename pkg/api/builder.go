package api

import (
	"context"
	"io"
	"reflect"

	"github.com/ipfs/testground/pkg/config"
)

// Builder is the interface to be implemented by all builders. A builder takes a
// test plan and builds it into executable form against a set of upstream
// dependencies, so it can be scheduled by a runner.
type Builder interface {
	// ID returns the canonical identifier for this builder.
	ID() string

	// Build performs a build.
	Build(ctx context.Context, input *BuildInput, output io.Writer) (*BuildOutput, error)

	// ConfigType returns the configuration type of this builder.
	ConfigType() reflect.Type
}

// BuildInput encapsulates the input options for building a test plan.
type BuildInput struct {
	// BuildID is a unique ID for this build.
	BuildID string
	// EnvConfig is the env configuration of the engine. Not a pointer to force
	// a copy.
	EnvConfig config.EnvConfig
	// Directories providers accessors to directories managed by the runtime.
	Directories Directories
	// TestPlan is the metadata of the test plan being built.
	TestPlan *TestPlanDefinition
	// Dependencies are the versions of upstream dependencies we want to build
	// against. For a go build, this could be e.g.:
	//  github.com/ipfs/go-ipfs=v0.4.22
	//  github.com/libp2p/go-libp2p=v0.2.8
	Dependencies map[string]string
	// BuildConfig is the configuration of the build job sourced from the test
	// plan manifest, coalesced with any user-provided overrides.
	BuildConfig interface{}
}

// BuildOutput encapsulates the output from a build action.
type BuildOutput struct {
	// BuilderID is the ID of the builder used.
	BuilderID string
	// ArtifactPath can be the docker image ID, a file location, etc. of the
	// resulting artifact. It is builder-dependent.
	ArtifactPath string
	// Dependencies is a map of modules (as keys) to versions (as values),
	// containing the collapsed transitive upstream dependency set of this
	// build.
	Dependencies map[string]string
}
