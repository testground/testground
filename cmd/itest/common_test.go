package cmd_test

import (
	"context"
	"testing"

	"github.com/ipfs/testground/cmd"
	"github.com/ipfs/testground/pkg/server"

	"github.com/urfave/cli"
)

func runSingle(t *testing.T, args ...string) error {
	t.Helper()

	srv, err := server.New("localhost:0")
	if err != nil {
		t.Fatal(err)
	}

	err = srv.Start()
	if err != nil {
		t.Fatal(err)
	}

	defer srv.Shutdown(context.Background()) //nolint

	app := cli.NewApp()
	app.Name = "testground"
	app.Commands = cmd.Commands
	app.Flags = cmd.Flags
	app.HideVersion = true

	args = append([]string{"testground", "--endpoint", srv.Addr()}, args...)
	return app.Run(args)
}
