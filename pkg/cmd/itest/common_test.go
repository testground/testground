package cmd_test

import (
	"context"
	"testing"

	"github.com/testground/testground/pkg/cmd"
	"github.com/testground/testground/pkg/config"
	"github.com/testground/testground/pkg/daemon"

	"github.com/urfave/cli/v2"
)

func runSingle(t *testing.T, args ...string) error {
	t.Helper()

	cfg := &config.EnvConfig{}
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

	args = append([]string{"testground", "--endpoint", srv.Addr()}, args...)
	return app.Run(args)
}
