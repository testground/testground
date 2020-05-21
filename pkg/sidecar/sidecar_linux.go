//+build linux

package sidecar

import (
	"context"
	"fmt"

	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/sync"
	"github.com/testground/testground/pkg/logging"
)

const (
	EnvRedisHost    = "REDIS_HOST"
	EnvInfluxdbHost = "INFLUXDB_HOST"
)

var runners = map[string]func() (Reactor, error){
	"docker": NewDockerReactor,
	"k8s":    NewK8sReactor,
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

// Run runs the sidecar in the given runner environment.
func Run(runnerName string) error {
	globalctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner, ok := runners[runnerName]
	if !ok {
		return fmt.Errorf("sidecar runner %s not found", runnerName)
	}

	reactor, err := runner()
	if err != nil {
		return fmt.Errorf("failed to initialize sidecar: %s", err)
	}

	logging.S().Infow("starting sidecar", "runner", runnerName)
	defer logging.S().Infow("stopping sidecar", "runner", runnerName)

	defer reactor.Close()

	// this call blocks.
	err = reactor.Handle(globalctx, handler)
	return err
}

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
	topic := sync.NewTopic("network"+instance.Hostname, network.Config{})
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
