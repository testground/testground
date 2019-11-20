package cmd

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	envcfg, err := config.GetEnvConfig()
	if err != nil {
		return err
	}

	if envcfg.Daemon.Listen == "" {
		logging.S().Fatal("missing daemon configuration. copy env-example.yaml to .env.toml and configure [daemon] section")
	}

	srv := server.New(envcfg.Daemon.Listen)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logging.S().Infow("listen and serve", "addr", srv.Addr)
		err := srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			logging.S().Fatalw("server exited", "err", err)
		}
	}()

	<-done
	logging.S().Infow("shutting down rpc server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logging.S().Fatalw("failed to shut down rpc server", "err", err)
	}
	logging.S().Infow("rpc server stopped")

	return nil
}
