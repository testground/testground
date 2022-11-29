//go:build integration && local_exec
// +build integration,local_exec

package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
	. "github.com/testground/testground/pkg/integration/utils"
)

func TestPlacebok(t *testing.T) {
	Setup(t)

	params := RunSingle{
		Plan:      "testground/placebo",
		Testcase:  "ok",
		Builder:   "exec:go",
		Runner:    "local:exec",
		Instances: 2,
		Collect:   true,
		Wait:      true,
	}

	result, err := Run(t, params)

	require.NoError(t, err)
	require.Equal(t, 0, result.ExitCode)
	require.NotEmpty(t, result.Stdout)
	// TODO: `RequireOutcomeIsSuccess(t, result)` -- At the moment the local:exec runner generate an unknown outcome.

	// TODO: port assert_run_output_is_correct (which checks the output content also).
}

// fix: go dependencies rewrite in exec:go
// https://github.com/testground/testground/pull/1469
func TestOverrideDependencies(t *testing.T) {
	Setup(t)

	params := RunComposition{
		File: "../../plans/placebo/_compositions/pr-1469-override-dependencies.toml",
		Runner: "local:exec",
		Collect: true,
		Wait: true,
	}

	result, err := RunAComposition(t, params)

	require.NoError(t, err)
	require.Equal(t, 0, result.ExitCode)
	require.NotEmpty(t, result.Stdout)
	// TODO: `RequireOutcomeIsSuccess(t, result)` -- At the moment the local:exec runner generate an unknown outcome.

	// TODO: port assert_run_output_is_correct (which checks the output content also).
}

