//go:build integration
// +build integration

package utils

import (
	"testing"

	"github.com/testground/testground/pkg/config"
	"github.com/testground/testground/pkg/daemon"
)

func setupDaemon(t *testing.T) *daemon.Daemon {
	t.Helper()

	cfg := &config.EnvConfig{
		Daemon: config.DaemonConfig{
			Scheduler: config.SchedulerConfig{
				TaskRepoType: "memory",
			},
			Listen: "localhost:0",
		},
	}
	if err := cfg.EnsureMinimalConfig(); err != nil {
		t.Fatal(err)
	}

	srv, err := daemon.New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	go srv.Serve() //nolint

	return srv
}
