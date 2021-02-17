package cmd

import (
	"context"
	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/sync"
	"github.com/urfave/cli/v2"
	"net/http"
	"os"
)

const defaultRedisHost = "testground-redis"
const envRedisHost = "REDIS_HOST"

var SyncCommand = cli.Command{
	Name:   "sync",
	Usage:  "run the sync server process",
	Action: syncCommand,
}

func syncCommand(c *cli.Context) error {
	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	redisHost := os.Getenv(envRedisHost)
	if redisHost == "" {
		redisHost = defaultRedisHost
	}

	service, err := sync.NewRedisService(ctx, logging.S(), &sync.RedisConfiguration{
		Port: 6379,
		Host: redisHost,
	})
	if err != nil {
		return err
	}
	service.EnableBackgroundGC(nil)

	srv, err := sync.NewServer(service, 5050)
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

		logging.S().Infow("shutting down sync service")

		_ = service.Close()
		_ = srv.Shutdown(ctx)
	}()

	logging.S().Infow("sync service listening", "addr", srv.Addr())
	err = srv.Serve()
	if err == http.ErrServerClosed {
		err = nil
	}
	return err
}
