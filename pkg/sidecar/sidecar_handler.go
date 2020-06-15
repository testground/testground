package sidecar

import (
	"context"
	"fmt"

	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/sync"
)

const (
	defaultDataNetwork = "default"
)

func handler(ctx context.Context, instance *Instance) error {
	instance.S().Debugw("managing instance", "instance", instance.Hostname)

	defer func() {
		instance.S().Debugw("closing instance", "instance", instance.Hostname)
		if err := instance.Close(); err != nil {
			instance.S().Warnf("failed to close instance: %s", err)
		}
	}()

	// Network configuration loop.
	err := instance.Network.ConfigureNetwork(ctx, &network.Config{
		Network: defaultDataNetwork,
		Enable:  true,
	})

	if err != nil {
		return err
	}

	ctx = sync.WithRunParams(ctx, &instance.RunEnv.RunParams)

	// Wait for all the sidecars to enter the "network-initialized" state.
	instance.S().Infof("waiting for all networks to be ready")

	const netInitState = "network-initialized"
	total := instance.RunEnv.TestInstanceCount
	if _, err := instance.Client.SignalAndWait(ctx, netInitState, total); err != nil {
		return fmt.Errorf("failed to signal network ready: %w", err)
	}

	instance.S().Infof("all networks ready")

	// Now let the test case tell us how to configure the network.
	topic := sync.NewTopic("network:"+instance.Hostname, network.Config{})
	networkChanges := make(chan *network.Config, 16)
	if _, err := instance.Client.Subscribe(ctx, topic, networkChanges); err != nil {
		return fmt.Errorf("failed to subscribe to network changes: %s", err)
	}

	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			if err != nil && err != context.Canceled {
				instance.S().Warnw("context return err different to canceled", "err", err.Error())
			}
			return nil

		case cfg, ok := <-networkChanges:
			if !ok {
				instance.S().Debugw("networkChanges channel closed", "instance", instance.Hostname)
				return nil
			}

			instance.S().Infow("applying network change", "network", cfg)
			if err := instance.Network.ConfigureNetwork(ctx, cfg); err != nil {
				return fmt.Errorf("failed to update network %s: %w", cfg.Network, err)
			}

			if cfg.CallbackState != "" {
				_, err := instance.Client.SignalEntry(ctx, cfg.CallbackState)
				if err != nil {
					return fmt.Errorf("failed to signal network state change %s: %w", cfg.CallbackState, err)
				}
			}
		}
	}
}
