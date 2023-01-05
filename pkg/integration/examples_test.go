//go:build integration && docker_examples
// +build integration,docker_examples

package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
	. "github.com/testground/testground/pkg/integration/utils"
)

func TestRustExample(t *testing.T) {
	Setup(t)

	DockerPull(
		t,
		"rust:1.59-bullseye",
	)

	params := RunSingleParams{
		Plan:      "testground/example-rust",
		Testcase:  "tcp-connect",
		Builder:   "docker:generic",
		Runner:    "local:docker",
		Instances: 2,
		Wait:      true,
	}

	result, err := RunSingle(t, params)

	require.NoError(t, err)
	require.Equal(t, 0, result.ExitCode)
	require.NotEmpty(t, result.Stdout)
	RequireOutcomeIsSuccess(t, result)
}

func TestNodeExample(t *testing.T) {
	Setup(t)

	params := RunSingleParams{
		Plan:      "testground/example-js",
		Testcase:  "pingpong",
		Builder:   "docker:node",
		Runner:    "local:docker",
		Instances: 2,
		Wait:      true,
	}

	result, err := RunSingle(t, params)

	require.NoError(t, err)
	require.Equal(t, 0, result.ExitCode)
	RequireOutcomeIsSuccess(t, result)
}

func TestGenericArtifact(t *testing.T) {
	Setup(t)

	params := RunSingleParams{
		Plan:      "testground/example",
		Testcase:  "artifact",
		Builder:   "docker:generic",
		Runner:    "local:docker",
		Instances: 1,
		Wait:      true,
	}

	result, err := RunSingle(t, params)

	require.NoError(t, err)
	require.Equal(t, 0, result.ExitCode)
	RequireOutcomeIsSuccess(t, result)
}

func TestExampleBrowser(t *testing.T) {
	Setup(t)

	DockerPull(
		t,
		"node:16",
		"mcr.microsoft.com/playwright:v1.25.2-focal",
	)

	cases := []struct {
		browser string
		testcase string
		instances int
		expectSuccess bool
	}{
		{"browser=chromium", "output", 1, true},
		{"browser=firefox", "output", 1, true},
		{"browser=webkit", "output", 1, true},
		{"browser=chromium", "failure", 1, false},
		{"browser=firefox", "failure", 1, false},
		{"browser=webkit", "failure", 1, false},
		{"browser=chromium", "sync", 2, true},
		{"browser=firefox", "sync", 2, true},
		{"browser=webkit", "sync", 2, true},
	}

	for _, c := range cases {
		params := RunSingleParams{
			Plan:      "testground/example-browser-node",
			Testcase:  c.testcase,
			Builder:   "docker:generic",
			Runner:    "local:docker",
			Instances: c.instances,
			Wait:      true,
			TestParams: []string{
				c.browser,
			},
		}

		result, err := RunSingle(t, params)

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

func TestExampleBrowserWithSyncAccrossRuntimes(t *testing.T) {
	Setup(t)

	params := RunCompositionParams{
		File: "../../plans/example-browser-node/compositions/sync-cross-runtime.toml",
		Runner: "local:docker",
		Wait: true,
	}

	result, err := RunComposition(t, params)

	require.NoError(t, err)
	require.Equal(t, 0, result.ExitCode)
	require.NotEmpty(t, result.Stdout)
}
