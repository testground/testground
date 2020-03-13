package cmd_test

import (
	"context"
	"testing"

	"github.com/ipfs/testground/cmd"
	"github.com/ipfs/testground/pkg/daemon"

	"github.com/urfave/cli"
)

func runSingle(t *testing.T, args ...string) error {
	t.Helper()
	t.Parallel()

	srv, err := daemon.New("localhost:0")
	if err != nil {
		t.Fatal(err)
	}

	go srv.Serve()                           //nolint
	defer srv.Shutdown(context.Background()) //nolint

	app := cli.NewApp()
	app.Name = "testground"
	app.Commands = cmd.Commands
	app.Flags = cmd.Flags
	app.HideVersion = true

	// Try to speed up travis ci by using a go proxy...
	args = append(args, "--build-cfg", "go_proxy_mode=remote", "--build-cfg", "go_proxy_url=https://proxy.golang.org")

	args = append([]string{"testground", "--endpoint", srv.Addr()}, args...)
	return app.Run(args)
}
