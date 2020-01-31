package cmd_test

import (
	"testing"
)

func XTestDescribeExistingPlan(t *testing.T) {
	err := runSingle(t,
		"describe",
		"placebo",
	)

	if err != nil {
		t.Fatal(err)
	}
}

func TestDescribeInexistentPlan(t *testing.T) {
	err := runSingle(t,
		"describe",
		"i-do-not-exist",
	)

	if err == nil {
		t.Fatal("expected to get an err, due to non-existing test plan, but got nil")
	}
}
