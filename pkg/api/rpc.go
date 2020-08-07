package api

import (
	"bytes"
)

// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
// ~~~~~~ Request payloads ~~~~~~
// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

// DescribeRequest is the request struct for the `describe` function.
type DescribeRequest struct {
	Term string `json:"term"`
}

// BuildRequest is the request struct for the `build` function.
type BuildRequest struct {
	Wait        bool             `json:"wait"`
	Composition Composition      `json:"composition"`
	Manifest    TestPlanManifest `json:"manifest"`
}

// RunRequest is the request struct for the `run` function.
type RunRequest struct {
	Wait        bool             `json:"wait"`
	BuildGroups []int            `json:"buildGroups"`
	Composition Composition      `json:"composition"`
	Manifest    TestPlanManifest `json:"manifest"`
}

type OutputsRequest struct {
	Runner string `json:"runner"`
	RunID  string `json:"run_id"`
}

type TerminateRequest struct {
	Runner  string `json:"runner"`
	Builder string `json:"builder"`
}

type HealthcheckRequest struct {
	Runner string `json:"runner"`
	Fix    bool   `json:"fix"`
}

type BuildPurgeRequest struct {
	Builder  string `json:"builder"`
	Testplan string `json:"testplan"`
}

// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
// ~~~~~~ Response payloads ~~~~~~
// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

// BuildResponse is the response struct for the `build` function.
type BuildResponse = []BuildOutput

type RunResponse = RunOutput

type CollectResponse struct {
	File   bytes.Buffer
	Exists bool
}

type HealthcheckResponse = HealthcheckReport
