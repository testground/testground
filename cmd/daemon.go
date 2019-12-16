package cmd

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ipfs/testground/pkg/config"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/pkg/server"
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

	var (
		listen = envcfg.Daemon.Listen
		srv    = server.New(listen)
	)

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

	fmt.Printf("daemon listening on addr: %s\n", srv.Addr)
	err = srv.ListenAndServe()
	if err == http.ErrServerClosed {
		err = nil
	}
	return err
}
