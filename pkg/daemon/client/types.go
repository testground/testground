package client

import "github.com/ipfs/testground/pkg/api"

type DescribeRequest struct {
	Term string `json:"term"`
}

type BuildRequest struct {
	Dependencies map[string]string `json:"deps"`
	BuildConfig  interface{}       `json:"build_config"`
	Plan         string            `json:"plan"`
	Builder      string            `json:"builder"`
}

type BuildResponse = api.BuildOutput

type RunRequest struct {
	Plan         string            `json:"plan"`
	Case         string            `json:"case"`
	Runner       string            `json:"runner"`
	Instances    int               `json:"instances"`
	ArtifactPath string            `json:"artifact_path"`
	Parameters   map[string]string `json:"parameters"`
	RunnerConfig interface{}       `json:"runner_config"`
}
