//go:build integration
// +build integration

package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testground/testground/pkg/task"
)

func GetMatchedGroups(regEx *regexp.Regexp, x string) map[string]string {
	match := regEx.FindStringSubmatch(x)

	if match == nil {
		return nil
	}

	group_names := regEx.SubexpNames()
	groups := make(map[string]string, len(group_names))

	for i, name := range group_names {
		if i > 0 && i <= len(match) {
			groups[name] = match[i]
		}
	}

	return groups
}

func getResultArtifact(t *testing.T, result *BuildResult) (string, error) {
	t.Helper()

	// Find the "generated artifact id" in the result's stdout.
	// `Jan  5 13:43:20.291394  INFO    generated build artifact        {"group": "single", "artifact": "88902381765a"}`
	// match_stdout := regexp.MustCompile("generated build artifact.*\"artifact\": \"(?P<artifact_id>[a-z0-9-]+)\".*")
	match_stdout := regexp.MustCompile("generated build artifact.*\"artifact\": \"(?P<artifact_id>[a-z0-9-]+)\".*")
	groups := GetMatchedGroups(match_stdout, result.Stdout)

	if groups == nil {
		return "", fmt.Errorf("Could not find artifact id in stdout: %s", result.Stdout)
	}

	return groups["artifact_id"], nil
}

func RequireOutcomeIs(t *testing.T, outcome task.Outcome, result *RunResult) {
	t.Helper()

	// Find the "outcome" in the result's stdout.
	// `run finished with outcome = failure (single:0/1)`
	// `run finished with outcome unknown`
	match_stdout := regexp.MustCompile("run finished with outcome (= )?(?P<outcome>[a-z0-9-]+)")
	groups := GetMatchedGroups(match_stdout, result.Stdout)

	if groups == nil {
		t.Fatalf("Could not find outcome in stdout: %s", result.Stdout)
	}

	require.Equal(t, string(outcome), groups["outcome"])
}

func RequireOutcomeIsSuccess(t *testing.T, result *RunResult) {
	RequireOutcomeIs(t, task.OutcomeSuccess, result)
}

func RequireOutcomeIsFailure(t *testing.T, result *RunResult) {
	RequireOutcomeIs(t, task.OutcomeFailure, result)
}

func RequireOutcomeIsCanceled(t *testing.T, result *RunResult) {
	RequireOutcomeIs(t, task.OutcomeCanceled, result)
}

/**
 * Require that the folder contains some data in `folder/run.out` and an empty `folder/run.err`.
 */
func requireOutputInstanceContainsAValidResult(t *testing.T, instanceCollectFolder string) {
	t.Helper()

	singleOutput := filepath.Join(instanceCollectFolder, "run.out")
	singleErr := filepath.Join(instanceCollectFolder, "run.err")

	// require file is NOT empty
	f, err := os.Stat(singleOutput)
	require.NoError(t, err)
	require.False(t, f.IsDir())
	require.NotZero(t, f.Size())

	// require file is empty
	f, err = os.Stat(singleErr)
	require.NoError(t, err)
	require.False(t, f.IsDir())
	require.Zero(t, f.Size())
}

// Requires that the folder collected from the run contains
// some logs and no errors.
// Assumes it is a single run for a single group.
func RequireOutputContainsASingleValidResult(t *testing.T, collectFolder string) {
	t.Helper()
	singleOutput := filepath.Join(collectFolder, "single", "0")
	requireOutputInstanceContainsAValidResult(t, singleOutput)
}

/**
* Assert that a testground run has no errors and some logs.
 */
func RequireRunOutputIsCorrect(t *testing.T, collectFolder string) {
	t.Helper()

	// In the collect folder we have `/[groupName]/[instanceId]/[outputs files]`.
	// Go through each subfolders and assert that the folder contains run.out and an empty run.err
	groupsFolders, err := ioutil.ReadDir(collectFolder)
	require.NoError(t, err)

	for _, f := range groupsFolders {
		if !f.IsDir() {
			continue
		}

		collectGroupFolder := filepath.Join(collectFolder, f.Name())

		// iterate through instance folders
		instanceFolders, err := ioutil.ReadDir(collectGroupFolder)
		require.NoError(t, err)

		for _, instanceFolder := range instanceFolders {
			if !instanceFolder.IsDir() {
				continue
			}

			requireOutputInstanceContainsAValidResult(t, filepath.Join(collectGroupFolder, instanceFolder.Name()))
		}
	}
}
