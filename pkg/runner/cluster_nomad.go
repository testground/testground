package runner

import (
	"reflect"

	"github.com/ipfs/testground/pkg/api"
)

type NomadRunner struct {
	// Endpoint URL is the endpoint of the Nomad control plane.
	EndpointURL string
}

var _ api.Runner = (*NomadRunner)(nil)

// TODO: NomadRunner.
func (*NomadRunner) Run(input *api.RunInput) (*api.RunOutput, error) {
	return nil, nil
}

func (*NomadRunner) ID() string {
	return "cluster:nomad"
}

func (*NomadRunner) ConfigType() reflect.Type {
	// TODO
	return nil
}

func (*NomadRunner) CompatibleBuilders() []string {
	// TODO
	return nil
}
