//go:build integration
// +build integration

package utils

import (
	"path/filepath"
)

type RunSingle struct {
	Plan       string
	Testcase   string
	Builder    string
	Runner     string
	Instances  int
	Collect    bool
	Wait       bool
	TestParams []string
}

type RunComposition struct {
	File string
	// TODO: this is how we implemented these tests before.
	// Load the composition directly and remove the need for this field.
	Runner  string
	Collect bool
	Wait    bool
}

type RunResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	CollectFolder string
	Cleanup func()
}

// (pure method) rewrite the composition parameters to use absolute paths.
func (r RunComposition) withAbsolutePath() RunComposition {
	newPath, err := filepath.Abs(r.File)

	if err != nil {
		panic(err)
	}

	r.File = newPath
	return r
}