package build

import (
	"testing"
)

func TestLocateBaseDir(t *testing.T) {
	t.Skip("this test is skipped under normal conditions, as go doesn't run it from the source tree; " +
		"enable for debugging")

	_, err := LocateBaseDir()
	if err != nil {
		t.Fatal(err)
	}
}
