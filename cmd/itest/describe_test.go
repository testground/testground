package cmd_test

import (
	"testing"
)

func TestDescribeExistingPlan(t *testing.T) {
	t.Log("running TestDescribeExistingPlan")
	err := runSingle(t,
		"describe",
		"placebo",
	)

	if err != nil {
		t.Fatal(err)
	}
}

func TestDescribeInexistentPlan(t *testing.T) {
	t.Log("running TestDescribeInexistentPlan")
	err := runSingle(t,
		"describe",
		"i-do-not-exist",
	)

	if err == nil {
		t.Fatal("expected to get an err, due to non-existing test plan, but got nil")
	}
}
