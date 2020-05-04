package sidecar

import (
	"context"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
	"math/rand"
	"testing"
	"time"
)

const (
	CONFIGUREDNETWORK   = "testNewNet"
	CONFIGUREDBANDWIDTH = uint64(12345)
)

func unique_runenv() (hostname string, runenv *runtime.RunEnv) {
	rand.Seed(time.Now().UnixNano())
	unique := string(rand.Int())
	params := runtime.RunParams{
		TestPlan:               "sidecarPlan",
		TestCase:               "sidecarCase",
		TestRun:                unique,
		TestInstanceCount:      1,
		TestInstanceRole:       "sidecarRole",
		TestGroupID:            "sidecarGroup",
		TestGroupInstanceCount: 1,
	}
	hostname = "sidecar.test." + unique
	runenv = runtime.NewRunEnv(params)
	return
}

func verifyDefaultNetwork(n *sync.NetworkConfig) bool {
	return n.Network == "default" && n.Enable
}

func verifyConfiguredNetwork(n *sync.NetworkConfig) bool {
	return n.Network == CONFIGUREDNETWORK && n.Enable && n.Default.Bandwidth == CONFIGUREDBANDWIDTH
}

// This function is a test plan which requests a network change.
func planRequestNetworkConfigure(ctx context.Context, runenv *runtime.RunEnv, hostname string, errs chan error, done chan int, doConfigure bool) {
	client, err := sync.NewBoundClient(ctx, runenv)
	if err != nil {
		errs <- err
		return
	}

	err = client.WaitNetworkInitialized(ctx, runenv)
	if err != nil {
		errs <- err
		return
	}

	netCfg := sync.NetworkConfig{
		Network: CONFIGUREDNETWORK,
		State:   "mynetworkdone",
		Default: sync.LinkShape{
			Bandwidth: CONFIGUREDBANDWIDTH,
		},
	}
	if doConfigure {
		topic := sync.NetworkTopic(hostname)
		client.MustPublishAndWait(ctx, topic, netCfg, "mynetworkdone", 1)
	}

	close(done)
}

// TestSidecarConfiguresDefaultNetwork and TestSidecarConfiguresRequestedNetwork verify that the
// sidecar handler function is working correctly. This function listens for events on the sync
// service and responds by passing network configurations to a network handler. In this test, a mock
// network handler is used to simply record what configurations are passed to it.
// TODO: these tests rely on the real sync service client, but they shouldn't.

// Verify when no additional configuration is performed, the default network is configured and enabled.
func TestSidecarConfiguresDefaultNetwork(t *testing.T) {
	hostname, runenv := unique_runenv()
	nw := NewMockNetwork()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	planErrs := make(chan error)
	planDone := make(chan int)

	go planRequestNetworkConfigure(ctx, runenv, hostname, planErrs, planDone, false)

	client, err := sync.NewBoundClient(ctx, runenv)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}
	inst, err := NewInstance(client, runenv, hostname, nw)
	if err != nil {
		t.Error(err)
		t.Fail()
	}
	go handler(ctx, inst)

	// If the plan returns an error, we should fail.
	// If the plan finishes, we should check that the network has been configured.
	for {
		select {
		case <-planDone:
			numConf := len(nw.Configured)
			if numConf != 1 {
				t.Error("Expected to be configured one time. Actual number of configures:", numConf)
			}
			if !verifyDefaultNetwork(nw.Configured[0]) {
				t.Error("the default network configuration is incorrect.")
			}
			return
		case err := <-planErrs:
			t.Error(err)
			t.Fail()
		}
	}
}

// Verify when no additional configuration is performed, the default network is configured and enabled.
func TestSidecarConfiguresRequestedNetwork(t *testing.T) {
	hostname, runenv := unique_runenv()
	nw := NewMockNetwork()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	planErrs := make(chan error)
	planDone := make(chan int)

	go planRequestNetworkConfigure(ctx, runenv, hostname, planErrs, planDone, false)

	client, err := sync.NewBoundClient(ctx, runenv)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}
	inst, err := NewInstance(client, runenv, hostname, nw)
	if err != nil {
		t.Error(err)
		t.Fail()
	}
	go handler(ctx, inst)

	// If the plan returns an error, we should fail.
	// If the plan finishes, we should check that the network has been configured.
	for {
		select {
		case <-planDone:
			numConf := len(nw.Configured)
			if numConf != 2 {
				t.Error("Expected to be configured twice. Actual number of configures:", numConf)
			}
			if !verifyConfiguredNetwork(nw.Configured[1]) {
				t.Error("the configured network configuration is incorrect.")
			}
			return
		case err := <-planErrs:
			t.Error(err)
			t.Fail()
		}
	}
}
