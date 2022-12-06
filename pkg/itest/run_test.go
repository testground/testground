//go:build integration && local_exec
// +build integration,local_exec

package cmd_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAbortedTestShouldFailLocal(t *testing.T) {
	err := runSingle(t,
		&terminateOpts{
			runner:  "local:exec",
			builder: "exec:go",
		},
		"run",
		"single",
		"--builder",
		"exec:go",
		"--runner",
		"local:exec",
		"--instances",
		"1",
		"--plan",
		"placebo",
		"--testcase",
		"abort",
		"--wait",
	)

	require.Error(t, err)
}

func TestAbortedTestShouldFailDocker(t *testing.T) {
	err := runSingle(t,
		&terminateOpts{
			runner:  "local:docker",
			builder: "docker:go",
		},
		"run",
		"single",
		"--builder",
		"docker:go",
		"--runner",
		"local:docker",
		"--instances",
		"1",
		"placebo:abort",
		"--wait",
	)

	require.Error(t, err)
}

func TestIncompatibleRun(t *testing.T) {
	err := runSingle(t,
		&terminateOpts{
			runner:  "local:docker",
			builder: "exec:go",
		},
		"run",
		"single",
		"--builder",
		"exec:go",
		"--runner",
		"local:docker",
		"--instances",
		"1",
		"--plan",
		"placebo",
		"--testcase",
		"ok",
		"--wait",
	)

	require.Error(t, err)
}

func TestCompatibleRunLocal(t *testing.T) {
	err := runSingle(t,
		&terminateOpts{
			runner:  "local:exec",
			builder: "exec:go",
		},
		"run",
		"single",
		"--builder",
		"exec:go",
		"--runner",
		"local:exec",
		"--instances",
		"1",
		"--plan",
		"placebo",
		"--testcase",
		"ok",
		"--wait",
	)

	if err != nil {
		t.Fatal(err)
	}
}
