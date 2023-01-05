//go:build integration
// +build integration

package utils

import (
	"path/filepath"
	"time"
)

type RunSingleParams struct {
	Plan          string
	Testcase      string
	Builder       string
	Runner        string
	UseBuild      string // TODO: implement
	Instances     int
	Collect       bool
	Wait          bool
	TestParams    []string
	DaemonTimeout time.Duration
}

type RunCompositionParams struct {
	File string
	// TODO: this is how we implemented these tests before.
	// Load the composition directly and remove the need for this field.
	Runner  string
	Collect bool
	Wait    bool
}

type RunResult struct {
	ExitCode      int
	Stdout        string
	Stderr        string
	CollectFolder string
	Cleanup       func()
}

// (pure method) rewrite the composition parameters to use absolute paths.
func (r RunCompositionParams) withAbsolutePath() RunCompositionParams {
	newPath, err := filepath.Abs(r.File)

	if err != nil {
		panic(err)
	}

	r.File = newPath
	return r
}

type BuildSingleParams struct {
	Plan    string
	Builder string
	Wait    bool
}

type BuildResult struct {
	ExitCode      int
	Stdout        string
	Stderr        string
	Cleanup       func()
	Artifact	  string
}