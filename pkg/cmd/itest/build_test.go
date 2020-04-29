package cmd_test

import (
	"testing"
)

func TestBuildExecGo(t *testing.T) {
	err := runSingle(t,
		"build",
		"single",
		"--builder",
		"exec:go",
		"placebo",
	)

	if err != nil {
		t.Fatal(err)
	}
}

func TestBuildDockerGo(t *testing.T) {
	// TODO: this test assumes that docker is running locally, and that we can
	// pick the .env.toml file this way, in case the user has defined a custom
	// docker endpoint. I don't think those assumptions stand.
	err := runSingle(t,
		"build",
		"single",
		"--builder",
		"docker:go",
		"placebo",
	)

	if err != nil {
		t.Fatal(err)
	}
}
