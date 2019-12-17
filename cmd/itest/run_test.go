package cmd_test

import (
	"testing"
)

func TestAbortedTestShouldFailLocal(t *testing.T) {
	err := runSingle(t,
		"run",
		"placebo/abort",
		"--builder",
		"exec:go",
		"--runner",
		"local:exec",
	)

	if err == nil {
		t.Fail()
	}
}

func TestAbortedTestShouldFailDocker(t *testing.T) {
	err := runSingle(t,
		"run",
		"placebo/abort",
		"--builder",
		"docker:go",
		"--runner",
		"local:docker",
	)

	if err == nil {
		t.Fail()
	}
}

func TestIncompatibleRun(t *testing.T) {
	err := runSingle(t,
		"run",
		"placebo/ok",
		"--builder",
		"exec:go",
		"--runner",
		"local:docker",
	)

	if err == nil {
		t.Fail()
	}
}

func TestCompatibleRun(t *testing.T) {
	err := runSingle(t,
		"run",
		"placebo/ok",
		"--builder",
		"exec:go",
		"--runner",
		"local:exec",
	)

	if err != nil {
		t.Fail()
	}
}
