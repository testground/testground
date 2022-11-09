//go:build integration
// +build integration

package cmd_test

import (
	"testing"
)

func TestDescribeExistingPlan(t *testing.T) {
	err := runSingle(t, nil,
		"describe",
		"--plan",
		"placebo",
	)

	if err != nil {
		t.Fatal(err)
	}
}

func TestDescribeInexistentPlan(t *testing.T) {
	err := runSingle(t, nil,
		"describe",
		"--plan",
		"i-do-not-exist",
	)

	if err == nil {
		t.Fatal("expected to get an err, due to non-existing test plan, but got nil")
	}
}
