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
	// TODO
	panic("unimplemented")
}

func (*NomadRunner) ID() string {
	return "cluster:nomad"
}

func (*NomadRunner) ConfigType() reflect.Type {
	// TODO
	panic("unimplemented")
}

func (*NomadRunner) CompatibleBuilders() []string {
	// TODO
	panic("unimplemented")
}
