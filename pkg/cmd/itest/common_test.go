package cmd_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/testground/testground/pkg/cmd"
	"github.com/testground/testground/pkg/config"
	"github.com/testground/testground/pkg/daemon"

	"github.com/urfave/cli/v2"
)

type terminateOpts struct {
	runner  string
	builder string
}

func runSingle(t *testing.T, opts *terminateOpts, args ...string) error {
	t.Helper()

	cfg := &config.EnvConfig{
		Daemon: config.DaemonConfig{
			Scheduler: config.SchedulerConfig{
				TaskRepoType: "memory",
			},
		},
	}
	if err := cfg.Load(); err != nil {
		t.Fatal(err)
	}
	cfg.Daemon.Listen = "localhost:0"
	srv, err := daemon.New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	go srv.Serve()                           //nolint
	defer srv.Shutdown(context.Background()) //nolint

	app := cli.NewApp()
	app.Name = "testground"
	app.Commands = cmd.RootCommands
	app.Flags = cmd.RootFlags
	app.HideVersion = true

	endpoint := fmt.Sprintf("http://%s", srv.Addr())

	args = append([]string{"testground", "--endpoint", endpoint}, args...)
	err = app.Run(args)

	if opts != nil {
		if opts.builder != "" {
			args = []string{"testground", "--endpoint", endpoint, "terminate", "--builder", opts.builder}
			_ = app.Run(args)
		}

		if opts.runner != "" {
			args = []string{"testground", "--endpoint", endpoint, "terminate", "--runner", opts.runner}
			_ = app.Run(args)
		}
	}

	return err
}
