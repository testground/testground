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

	params := RunSingle{
		Plan:      "testground/example-rust",
		Testcase:  "tcp-connect",
		Builder:   "docker:generic",
		Runner:    "local:docker",
		Instances: 2,
		Collect:   true,
		Wait:      true,
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
		Plan:      "testground/example-js",
		Testcase:  "pingpong",
		Builder:   "docker:node",
		Runner:    "local:docker",
		Instances: 2,
		Collect:   true,
		Wait:      true,
	}

	result, err := Run(t, params)

	require.NoError(t, err)
	require.Equal(t, 0, result.ExitCode)
	RequireOutcomeIsSuccess(t, result)
}

func TestGenericArtifact(t *testing.T) {
	Setup(t)

	params := RunSingle{
		Plan:      "testground/example",
		Testcase:  "artifact",
		Builder:   "docker:generic",
		Runner:    "local:docker",
		Instances: 1,
		Collect:   true,
		Wait:      true,
	}

	result, err := Run(t, params)

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
		expectSuccess bool
	}{
		{"browser=chromium", "success", true},
		{"browser=firefox", "success", true},
		{"browser=chromium", "failure", false},
		{"browser=firefox", "failure", false},
	}

	for _, c := range cases {
		params := RunSingle{
			Plan:      "testground/example-browser",
			Testcase:  c.testcase,
			Builder:   "docker:generic",
			Runner:    "local:docker",
			Instances: 1,
			Collect:   true,
			Wait:      true,
			TestParams: []string{
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
