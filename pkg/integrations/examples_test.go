//go:build integration && docker_examples
// +build integration,docker_examples

package integrations

import (
	"testing"
	"github.com/stretchr/testify/require"
)

func Setup(t *testing.T) {
	t.Helper()

	err := runImport(t, "./plans", "testground")
	if err != nil {
		t.Fatal(err)
	}
}

func TestRustExample(t *testing.T) {
	Setup(t)

	params := RunSingle{
		plan:      "testground/example-rust",
		testcase:  "tcp-connect",
		builder:   "docker:generic",
		runner:    "local:docker",
		instances: 2,
		collect:   true,
		wait:      true,
	}

	result, err := Run(t, params)

	require.NoError(t, err)
	require.Equal(t, 0, result.ExitCode)
}

func TestNodeExample(t *testing.T) {
	Setup(t)

	params := RunSingle{
		plan:      "testground/example-js",
		testcase:  "pingpong",
		builder:   "docker:node",
		runner:    "local:docker",
		instances: 2,
		collect:   true,
		wait:      true,
	}

	result, err := Run(t, params)

	require.NoError(t, err)
	require.Equal(t, 0, result.ExitCode)
}

func TestGenericArtifact(t *testing.T) {
	Setup(t)

	params := RunSingle{
		plan:      "testground/example",
		testcase:  "artifact",
		builder:   "docker:generic",
		runner:    "local:docker",
		instances: 1,
		collect:   true,
		wait:      true,
	}

	result, err := Run(t, params)

	require.NoError(t, err)
	require.Equal(t, 0, result.ExitCode)
}
