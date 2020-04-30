package cmd_test

import (
	"testing"
)

func TestAbortedTestShouldFailLocal(t *testing.T) {
	err := runSingle(t,
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
	)

	if err == nil {
		t.Fail()
	}
}

func TestAbortedTestShouldFailDocker(t *testing.T) {
	err := runSingle(t,
		"run",
		"single",
		"--builder",
		"docker:go",
		"--runner",
		"local:docker",
		"--instances",
		"1",
		"placebo:abort",
	)

	if err == nil {
		t.Fail()
	}
}

func TestIncompatibleRun(t *testing.T) {
	err := runSingle(t,
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
	)

	if err == nil {
		t.Fail()
	}
}

func TestCompatibleRunLocal(t *testing.T) {
	err := runSingle(t,
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
	)

	if err != nil {
		t.Fail()
	}
}

func TestCompatibleRunDocker(t *testing.T) {
	err := runSingle(t,
		"run",
		"single",
		"--builder",
		"docker:go",
		"--runner",
		"local:docker",
		"--instances",
		"1",
		"--plan",
		"placebo",
		"--testcase",
		"ok",
	)

	if err != nil {
		t.Fail()
	}
}
