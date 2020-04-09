package client

import (
	"bytes"

	"github.com/ipfs/testground/pkg/api"
)

// DescribeRequest is the request struct for the `describe` function.
type DescribeRequest struct {
	Term string `json:"term"`
}

// BuildRequest is the request struct for the `build` function.
type BuildRequest struct {
	Composition api.Composition `json:"composition"`
}

// BuildResponse is the response struct for the `build` function.
type BuildResponse = []api.BuildOutput

// RunRequest is the request struct for the `run` function.
type RunRequest struct {
	Composition api.Composition `json:"composition"`
}

type RunResponse = api.RunOutput

type CollectResponse struct {
	File   bytes.Buffer
	Exists bool
}

type OutputsRequest struct {
	Composition api.Composition `json:"composition"`
	Runner      string          `json:"runner"`
	RunID       string          `json:"run_id"`
}

type TerminateRequest struct {
	Runner string `json:"runner"`
}

type HealthcheckRequest struct {
	Runner string `json:"runner"`
	Fix    bool   `json:"fix"`
}

type HealthcheckResponse = api.HealthcheckReport
