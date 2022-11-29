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

	dockerPull(
		t,
		"rust:1.59-bullseye",
	)

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
	require.NotEmpty(t, result.Stdout)
	RequireOutcomeIsSuccess(t, result)
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
	RequireOutcomeIsSuccess(t, result)
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
	RequireOutcomeIsSuccess(t, result)
}

func TestExampleBrowser(t *testing.T) {
	Setup(t)

	dockerPull(
		t,
		"node:16",
		"mcr.microsoft.com/playwright:v1.25.2-focal",
	)

	cases := []struct {
		browser string
		testcase string
		expectSuccess bool
	}{
		{"browser=chromium", "success", true},
		{"browser=firefox", "success", true},
		{"browser=chromium", "failure", false},
		{"browser=firefox", "failure", false},
	}

	for _, c := range cases {
		params := RunSingle{
			plan:      "testground/example-browser",
			testcase:  c.testcase,
			builder:   "docker:generic",
			runner:    "local:docker",
			instances: 1,
			collect:   true,
			wait:      true,
			testParams: []string{
				c.browser,
			},
		}

		result, err := Run(t, params)

		if c.expectSuccess {
			require.NoError(t, err)
			require.Equal(t, 0, result.ExitCode)
			RequireOutcomeIsSuccess(t, result)
		} else {
			require.Error(t, err)
			require.Equal(t, 1, result.ExitCode)
			RequireOutcomeIsFailure(t, result)
		}
	}
}
