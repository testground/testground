package cmd_test

import (
	"testing"
)

func XTestAbortedTestShouldFailLocal(t *testing.T) {
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

func XTestAbortedTestShouldFailDocker(t *testing.T) {
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

func XTestIncompatibleRun(t *testing.T) {
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

func XTestCompatibleRunLocal(t *testing.T) {
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

func XTestCompatibleRunDocker(t *testing.T) {
	err := runSingle(t,
		"run",
		"placebo/ok",
		"--builder",
		"docker:go",
		"--runner",
		"local:docker",
	)

	if err != nil {
		t.Fail()
	}
}
