//go:build integration && local_exec
// +build integration,local_exec

package integrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlacebok(t *testing.T) {
	Setup(t)

	params := RunSingle{
		plan:      "testground/placebo",
		testcase:  "ok",
		builder:   "exec:go",
		runner:    "local:exec",
		instances: 2,
		collect:   true,
		wait:      true,
	}

	result, err := Run(t, params)

	require.NoError(t, err)
	require.Equal(t, 0, result.ExitCode)
	require.NotEmpty(t, result.Stdout)
	RequireOutcomeIsSuccess(t, result)

	// TODO: port assert_run_output_is_correct (which checks the output content also).
}

// fix: go dependencies rewrite in exec:go
// https://github.com/testground/testground/pull/1469
func TestOverrideDependencies(t *testing.T) {
	Setup(t)

	params := RunComposition{
		file: "../../plans/placebo/_compositions/pr-1469-override-dependencies.toml",
		runner: "local:exec",
		collect: true,
		wait: true,
	}

	result, err := RunAComposition(t, params)

	require.NoError(t, err)
	require.Equal(t, 0, result.ExitCode)
	require.NotEmpty(t, result.Stdout)
	RequireOutcomeIsSuccess(t, result)

	// TODO: port assert_run_output_is_correct (which checks the output content also).
}

