//go:build linux
// +build linux

package sidecar

import (
	"context"
	"fmt"

	"github.com/testground/testground/pkg/logging"
)

const (
	EnvRedisHost       = "REDIS_HOST" // NOTE: kept for backwards compatibility with older SDKs.
	EnvSyncServiceHost = "SYNC_SERVICE_HOST"
	EnvInfluxdbHost    = "INFLUXDB_HOST"
	EnvAdditionalHosts = "ADDITIONAL_HOSTS"
)

var runners = map[string]func() (Reactor, error){
	"docker": NewDockerReactor,
	"k8s":    NewK8sReactor,
	"mock":   NewMockReactor,
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
