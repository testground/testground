package cmd_test

import (
	"testing"
)

func TestSidecar(t *testing.T) {
	if testing.Short() {
		return
	}
	err := runSingle(t,
		"run",
		"single",
		"network/ping-pong",
		"--builder",
		"docker:go",
		"--runner",
		"local:docker",
		"--instances",
		"2",
		//		"--build-cfg",
		//		"go_proxy_mode=remote",
		//		"--build-cfg",
		//		"go_proxy_url=https://proxy.golang.org",
	)

	if err != nil {
		t.Fail()
	}
}
