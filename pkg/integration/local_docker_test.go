//go:build integration && local_docker
// +build integration,local_docker

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	. "github.com/testground/testground/pkg/integration/utils"
)

// fix: stalled tests
// https://github.com/testground/testground/issues/1542
func TestPanickingTestWillEndAnyway(t *testing.T) {
	Setup(t)

	params := RunSingleParams{
		Plan:      "testground/_integrations",
		Testcase:  "issue-1542-stalled-test-panic",
		Builder:   "docker:go",
		Runner:    "local:docker",
		Instances: 2,
		Collect:   true,
		Wait:      true,
		DaemonTimeout: 2 * time.Minute,
	}

	result, err := RunSingle(t, params)
	defer result.Cleanup()

	require.Error(t, err)
	require.Equal(t, 1, result.ExitCode)
	require.NotEmpty(t, result.Stdout)

	RequireOutcomeIsFailure(t, result)
}

// fix: stalled tests
// https://github.com/testground/testground/issues/1542
func TestStalledTestWillEndAnyway(t *testing.T) {
	Setup(t)

	params := RunSingleParams{
		Plan:      "testground/_integrations",
		Testcase:  "issue-1542-stalled-test-stall",
		Builder:   "docker:go",
		Runner:    "local:docker",
		Instances: 2,
		Collect:   true,
		Wait:      true,
		DaemonTimeout: 1 * time.Minute,
	}

	result, err := RunSingle(t, params)
	defer result.Cleanup()

	require.Error(t, err)
	require.Equal(t, 1, result.ExitCode)
	require.NotEmpty(t, result.Stdout)

	RequireOutcomeIsFailure(t, result)

	require.Contains(t, result.Stdout, "run canceled after reaching the task timeout")
}