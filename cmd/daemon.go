package cmd

import (
	"context"
	"github.com/ipfs/testground/pkg/server"
	"net/http"
	"time"

	"github.com/ipfs/testground/pkg/config"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/urfave/cli"
)

// DaemonCommand is the specification of the `daemon` command.
var DaemonCommand = cli.Command{
	Name:   "daemon",
	Usage:  "start a long-running daemon process",
	Action: daemonCommand,
}

func daemonCommand(c *cli.Context) error {
	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	envcfg, err := config.GetEnvConfig()
	if err != nil {
		return err
	}

	listen := envcfg.Daemon.Listen
	srv, err := server.New(listen)
	if err != nil {
		return err
	}

	exiting := make(chan struct{})
	defer close(exiting)

	go func() {
		select {
		case <-ctx.Done():
		case <-exiting:
			// no need to shutdown in this case.
			return
		}

		logging.S().Infow("shutting down rpc server")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			logging.S().Fatalw("failed to shut down rpc server", "err", err)
		}
		logging.S().Infow("rpc server stopped")
	}()

	logging.S().Infow("listen and serve", "addr", srv.Addr)
	err = srv.Start()
	if err == http.ErrServerClosed {
		err = nil
	}
	return err
}
