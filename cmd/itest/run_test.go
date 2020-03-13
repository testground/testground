package cmd_test

import (
	"testing"
)

func TestAbortedTestShouldFailLocal(t *testing.T) {
	if testing.Short() {
		return
	}
	err := runSingle(t,
		"run",
		"single",
		"placebo/abort",
		"--builder",
		"exec:go",
		"--runner",
		"local:exec",
		"--instances",
		"1",
		"--build-cfg",
		"go_proxy_mode=remote",
		"--build-cfg",
		"go_proxy_url=https://proxy.golang.org",
	)

	if err == nil {
		t.Fail()
	}
}

func TestAbortedTestShouldFailDocker(t *testing.T) {
	if testing.Short() {
		return
	}
	err := runSingle(t,
		"run",
		"single",
		"placebo/abort",
		"--builder",
		"docker:go",
		"--runner",
		"local:docker",
		"--instances",
		"1",
		"--build-cfg",
		"go_proxy_mode=remote",
		"--build-cfg",
		"go_proxy_url=https://proxy.golang.org",
	)

	if err == nil {
		t.Fail()
	}
}

func TestIncompatibleRun(t *testing.T) {
	if testing.Short() {
		return
	}
	err := runSingle(t,
		"run",
		"single",
		"placebo/ok",
		"--builder",
		"exec:go",
		"--runner",
		"local:docker",
		"--instances",
		"1",
		"--build-cfg",
		"go_proxy_mode=remote",
		"--build-cfg",
		"go_proxy_url=https://proxy.golang.org",
	)

	if err == nil {
		t.Fail()
	}
}

func TestCompatibleRunLocal(t *testing.T) {
	if testing.Short() {
		return
	}
	err := runSingle(t,
		"run",
		"single",
		"placebo/ok",
		"--builder",
		"exec:go",
		"--runner",
		"local:exec",
		"--instances",
		"1",
		"--build-cfg",
		"go_proxy_mode=remote",
		"--build-cfg",
		"go_proxy_url=https://proxy.golang.org",
	)

	if err != nil {
		t.Fail()
	}
}

func TestCompatibleRunDocker(t *testing.T) {
	if testing.Short() {
		return
	}
	err := runSingle(t,
		"run",
		"single",
		"placebo/ok",
		"--builder",
		"docker:go",
		"--runner",
		"local:docker",
		"--instances",
		"1",
		"--build-cfg",
		"go_proxy_mode=remote",
		"--build-cfg",
		"go_proxy_url=https://proxy.golang.org",
	)

	if err != nil {
		t.Fail()
	}
}
