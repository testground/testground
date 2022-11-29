//go:build integration
// +build integration

package utils

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testground/testground/pkg/task"
)

func getMatchedGroups(regEx *regexp.Regexp, x string) map[string]string {
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

func RequireOutcomeIs(t *testing.T, outcome task.Outcome, result *RunResult) {
	t.Helper()

	// Find the string "outcome" in the result's stdout.
	// run finished with outcome = failure (single:0/1)
	match_stdout := regexp.MustCompile("run finished with outcome (= )?(?P<outcome>[a-z0-9-]+)")
	groups := getMatchedGroups(match_stdout, result.Stdout)

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
