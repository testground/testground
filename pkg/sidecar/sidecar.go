//+build linux

package sidecar

import (
	"context"
	"fmt"
	"io"

	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/sdk/sync"
)

var runners = map[string]func() (InstanceManager, error){
	"docker": NewDockerManager,
	"k8s":    NewK8sManager,
	// TODO: local
}

// GetRunners lists the available sidecar environments.
func GetRunners() []string {
	names := make([]string, 0, len(runners))
	for r := range runners {
		names = append(names, r)
	}
	return names
}

type InstanceManager interface {
	io.Closer
	Manage(context.Context, func(context.Context, *Instance) error) error
}

// Run runs the sidecar in the given runner environment.
func Run(runnerName string) error {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	runner, ok := runners[runnerName]
	if !ok {
		return fmt.Errorf("sidecar runner %s not found", runnerName)
	}

	manager, err := runner()
	if err != nil {
		return fmt.Errorf("failed to initialize sidecar: %s", err)
	}

	logging.S().Infow("starting sidecar", "runner", runnerName)

	defer manager.Close()

	return manager.Manage(ctx, func(ctx context.Context, instance *Instance) error {
		defer func() {
			if err := instance.Close(); err != nil {
				instance.S().Warnf("failed to close instance: %s", err)
			}
		}()

		err := instance.Network.ConfigureNetwork(ctx, &sync.NetworkConfig{
			Network: "default",
			Enable:  true,
		})
		if err != nil {
			return err
		}

		/* TODO: Initialize all networks to the "down" state.
		for _, n := range instance.Network.ListActive() {
			instance.Network.ConfigureNetwork(&sync.NetworkConfig{
				Network: n,
				Enable:  false,
			})
		}
		*/

		// Wait for all the sidecars to enter the "network-initialized" state.
		const netInitState = "network-initialized"
		if _, err = instance.Writer.SignalEntry(netInitState); err != nil {
			return fmt.Errorf("failed to signal network ready: %w", err)
		}
		instance.S().Infof("waiting for all networks to be ready")
		if err := <-instance.Watcher.Barrier(
			ctx,
			netInitState,
			int64(instance.RunEnv.TestInstanceCount),
		); err != nil {
			return fmt.Errorf("failed to wait for network ready: %w", err)
		}
		instance.S().Infof("all networks ready")

		// Now let the test case tell us how to configure the network.
		subtree := sync.NetworkSubtree(instance.Hostname)
		networkChanges := make(chan *sync.NetworkConfig, 16)
		closeSub, err := instance.Watcher.Subscribe(subtree, networkChanges)
		if err != nil {
			return fmt.Errorf("failed to subscribe to network changes: %s", err)
		}
		defer func() {
			if err := closeSub(); err != nil {
				instance.S().Warnf("failed to close sub: %s", err)
			}
		}()

		for cfg := range networkChanges {
			instance.S().Infow("applying network change", "network", cfg)
			if err := instance.Network.ConfigureNetwork(ctx, cfg); err != nil {
				return fmt.Errorf("failed to update network %s: %w", cfg.Network, err)
			}
			if cfg.State != "" {
				_, err := instance.Writer.SignalEntry(cfg.State)
				if err != nil {
					return fmt.Errorf(
						"failed to signal network state change %s: %w",
						cfg.State,
						err,
					)
				}
			}
		}

		return nil
	})
}
