//go:build linux
// +build linux

package cmd_test

import (
	"testing"
)

func TestList(t *testing.T) {
	err := runSingle(t, nil,
		"plan",
		"list",
	)

	if err != nil {
		t.Fatal(err)
	}
}
