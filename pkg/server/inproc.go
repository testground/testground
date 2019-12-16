package server

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/ipfs/testground/pkg/logging"
)

// ListenAndServe starts an in-process server on a random port
func ListenAndServe(ctx context.Context) (string, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return "", err
	}

	srv := New(listener.Addr().String())

	exiting := make(chan struct{})
	go func() {
		defer close(exiting)
		logging.S().Infow("listen and serve", "addr", srv.Addr)
		err := srv.Serve(listener)
		if err != nil && err != http.ErrServerClosed {
			logging.S().Fatalw("server exited", "err", err)
		}
	}()

	go func() {
		select {
		case <-ctx.Done():
		case <-exiting:
			// already stopped.
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

	return listener.Addr().String(), nil
}
