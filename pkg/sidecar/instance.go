//+build linux

package sidecar

import (
	"context"
	"fmt"
	"io"

	"github.com/hashicorp/go-multierror"

	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

type Instance struct {
	logging.Logging

	Hostname string
	Watcher  *sync.Watcher
	Writer   *sync.Writer
	RunEnv   *runtime.RunEnv
	Network  Network
}

type Network interface {
	io.Closer
	ConfigureNetwork(ctx context.Context, cfg *sync.NetworkConfig) error
	ListActive() []string
	ListAvailable() []string
}

func NewInstance(runenv *runtime.RunEnv, hostname string, network Network) (*Instance, error) {
	// Get a redis reader/writer.
	watcher, writer, err := sync.WatcherWriter(runenv)
	if err != nil {
		return nil, fmt.Errorf("during sync.WatcherWriter: %w", err)
	}

	return &Instance{
		Logging:  logging.NewLogging(runenv.SLogger().With("sidecar", true).Desugar()),
		Hostname: hostname,
		RunEnv:   runenv,
		Network:  network,
		Watcher:  watcher,
		Writer:   writer,
	}, nil
}

// Close closes the instance. It should not be used after closing.
func (inst *Instance) Close() error {
	var err *multierror.Error
	err = multierror.Append(err, inst.Watcher.Close())
	err = multierror.Append(err, inst.Writer.Close())
	err = multierror.Append(err, inst.Network.Close())
	return err.ErrorOrNil()
}
