//go:build integration && local_exec
// +build integration,local_exec

package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
	. "github.com/testground/testground/pkg/integration/utils"
)

func TestPlaceboIsOk(t *testing.T) {
	Setup(t)

	params := RunSingleParams{
		Plan:      "testground/placebo",
		Testcase:  "ok",
		Builder:   "exec:go",
		Runner:    "local:exec",
		Instances: 2,
		Collect:   true,
		Wait:      true,
	}

	result, err := RunSingle(t, params)
	defer result.Cleanup()

	require.NoError(t, err)
	require.Equal(t, 0, result.ExitCode)
	require.NotEmpty(t, result.Stdout)

	RequireOutputContainsASingleValidResult(t, result.CollectFolder)
}

// fix: go dependencies rewrite in exec:go
// https://github.com/testground/testground/pull/1469
func TestOverrideDependencies(t *testing.T) {
	Setup(t)

	params := RunCompositionParams{
		File: "../../plans/placebo/_compositions/pr-1469-override-dependencies.toml",
		Runner: "local:exec",
		Wait: true,
	}

	result, err := RunComposition(t, params)

	require.NoError(t, err)
	require.Equal(t, 0, result.ExitCode)
	require.NotEmpty(t, result.Stdout)
}

