package cmd

import (
	"context"
	"net/http"
	"time"

	"github.com/testground/testground/pkg/config"
	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/service"

	"github.com/urfave/cli/v2"
)

// DaemonCommand is the specification of the `daemon` command.
var ServiceCommand = cli.Command{
	Name:   "service",
	Usage:  "testground submit queue",
	Action: serviceCommand,
}

func serviceCommand(c *cli.Context) error {
	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	cfg := &config.EnvConfig{}
	if err := cfg.Load(); err != nil {
		return err
	}

	srv, err := service.New(cfg)
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

	logging.S().Infow("listen and serve", "addr", srv.Addr())
	err = srv.Serve()
	if err == http.ErrServerClosed {
		err = nil
	}
	return err
}
