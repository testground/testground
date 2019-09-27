package runner

import (
	"reflect"
)

type NomadRunner struct {
	// Endpoint URL is the endpoint of the Nomad control plane.
	EndpointURL string
}

var _ Runner = (*NomadRunner)(nil)

// TODO: NomadRunner.
func (*NomadRunner) Run(input *Input) (*Output, error) {
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
