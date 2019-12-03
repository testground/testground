package client

import "github.com/ipfs/testground/pkg/api"

// DescribeRequest is the request struct for the `describe` function
type DescribeRequest struct {
	Term string `json:"term"`
}

// BuildRequest is the request struct for the `build` function
type BuildRequest struct {
	Dependencies map[string]string `json:"deps"`
	BuildConfig  interface{}       `json:"build_config"`
	Plan         string            `json:"plan"`
	Builder      string            `json:"builder"`
}

// BuildResponse is the response struct for the `build` function
type BuildResponse = api.BuildOutput

// RunRequest is the request struct for the `run` function
type RunRequest struct {
	Plan         string            `json:"plan"`
	Case         string            `json:"case"`
	Runner       string            `json:"runner"`
	Instances    int               `json:"instances"`
	ArtifactPath string            `json:"artifact_path"`
	Parameters   map[string]string `json:"parameters"`
	RunnerConfig interface{}       `json:"runner_config"`
}
