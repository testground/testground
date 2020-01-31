package cmd_test

import (
	"testing"
)

func XTestSidecar(t *testing.T) {
	err := runSingle(t,
		"run",
		"network/ping-pong",
		"--builder",
		"docker:go",
		"--runner",
		"local:docker",
	)

	if err != nil {
		t.Fail()
	}
}
