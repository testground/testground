package runner

type NomadRunner struct {
	// Endpoint URL is the endpoint of the Nomad control plane.
	EndpointURL string
}

var _ Runner = (*NomadRunner)(nil)

// TODO: NomadRunner.
func (*NomadRunner) Run(input *Input, cfg interface{}) (*Output, error) {
	return nil, nil
}
