package cmd_test

import (
	"testing"
)

func TestBuildExecGo(t *testing.T) {
	if testing.Short() {
		return
	}
	err := runSingle(t,
		"build",
		"single",
		"placebo",
		"--builder",
		"exec:go",
		"--build-cfg",
		"go_proxy_mode=remote",
		"--build-cfg",
		"go_proxy_url=http://travis-goproxy:8081",
	)

	if err != nil {
		t.Fatal(err)
	}
}

func TestBuildDockerGo(t *testing.T) {
	// TODO: this test assumes that docker is running locally, and that we can
	// pick the .env.toml file this way, in case the user has defined a custom
	// docker endpoint. I don't think those assumptions stand.
	if testing.Short() {
		return
	}
	err := runSingle(t,
		"build",
		"single",
		"placebo",
		"--builder",
		"docker:go",
		"--build-cfg",
		"go_proxy_mode=remote",
		"--build-cfg",
		"go_proxy_url=http://travis-goproxy:8081",
	)

	if err != nil {
		t.Fatal(err)
	}
}
