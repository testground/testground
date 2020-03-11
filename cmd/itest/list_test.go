package cmd_test

import (
	"testing"
)

func TestList(t *testing.T) {
	t.Log("running TestList")
	err := runSingle(t,
		"list",
	)

	if err != nil {
		t.Fatal(err)
	}
}
