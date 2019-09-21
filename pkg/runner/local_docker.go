package runner

type LocalDockerRunner struct{}

var _ Runner = (*LocalDockerRunner)(nil)

func (*LocalDockerRunner) Run(input *Input, cfg interface{}) (*Output, error) {
	return nil, nil
}
