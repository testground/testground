package sidecar

import (
	"context"
	_ "context"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	sdknw "github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/runtime"
	sdksync "github.com/testground/sdk-go/sync"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

const (
	CONFIGUREDNETWORK   = "testtesttest_net"
	CONFIGUREDBANDWIDTH = uint64(12345)
	NETWORKDONE         = "configureddone"
)

func unique_runenv() (hostname string, runenv *runtime.RunEnv) {
	unique := strconv.Itoa(rand.Int())
	params := runtime.RunParams{
		TestPlan:               "sidecarPlan" + unique,
		TestCase:               "sidecarCase" + unique,
		TestRun:                unique,
		TestInstanceCount:      1,
		TestInstanceRole:       "sidecarRole" + unique,
		TestGroupID:            "sidecarGroup" + unique,
		TestGroupInstanceCount: 1,
	}
	hostname = "sidecar.test." + unique
	runenv = runtime.NewRunEnv(params)
	return
}

func verifyDefaultNetwork(t *testing.T, n *MockNetwork) {
	assert.Len(t, n.Configured, 1, "expected exactly one network to be configured")
	def := n.Configured[0]
	assert.Equal(t, "default", def.Network, "expected default network to be named `default`")
}

func verifyConfiguredNetwork(t *testing.T, n *MockNetwork) {
	assert.Len(t, n.Configured, 2, "expected exactly two networks to be configured")
	con := n.Configured[1]
	assert.Equal(t, CONFIGUREDNETWORK, con.Network, "expected configured network name to be passed to network driver")
	assert.Equal(t, CONFIGUREDBANDWIDTH, con.Default.Bandwidth, "expected requested bandwidth to be set")
}

// planRequestNetworkConfigure is a testground plan.
// It requests a change to the network configuration via the sync service.

/*
func planRequestNetworkConfigure(ctx context.Context, runenv *runtime.RunEnv, hostname string, done chan int, doConfigure bool) (err error) {
	client, err := sync.NewBoundClient(ctx, runenv)
	if err != nil {
		return
	}

	err = client.WaitNetworkInitialized(ctx, runenv)
	if err != nil {
		return
	}

	if doConfigure {
		netCfg := sync.NetworkConfig{
			Network: CONFIGUREDNETWORK,
			State:   NETWORKDONE,
			Default: sync.LinkShape{
				Bandwidth: CONFIGUREDBANDWIDTH,
			},
		}
		topic := sync.NetworkTopic(hostname)
		client.MustPublishAndWait(ctx, topic, netCfg, NETWORKDONE, 1)
	}

	time.Sleep(time.Second)
	close(done)
	return nil
}
*/

/*
// subtestSidecarNetworking starts planRequstNetworkConfigure and the real sidecar handler.
// The sidecar handler is setup to use a mock network so we can capture what would be passed to a
// real docker or kubernetes network.
func subtestSidecarNetworking(validator func(*testing.T, *MockNetwork), doConfigure bool) func(*testing.T) {
	return func(t *testing.T) {
		hostname, runenv := unique_runenv()
		nw := NewMockNetwork()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		errsCh := make(chan error)
		planDone := make(chan int)

		// Create Sidecar instance
		client, err := sync.NewBoundClient(ctx, runenv)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		inst, err := NewInstance(client, runenv, hostname, nw)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}

		// Start the plan
		go func() {
			errsCh <- planRequestNetworkConfigure(ctx, runenv, hostname, planDone, doConfigure)
		}()

		// Start sidecar handler
		go func() {
			errsCh <- handler(ctx, inst)
		}()

		// Wait until the plan finishes or an error occurs.
		for {
			select {
			case <-planDone:
				validator(t, nw)
				return
			case err := <-errsCh:
				if err != nil {
					t.Error(err)
					t.FailNow()
				}
			}
		}
	}
}


func TestSidecarConfiguresNetworking(t *testing.T) {
	// Don't configure the network, accepting the defaults.
	t.Run("DefaultNetworkConfiguration", subtestSidecarNetworking(verifyDefaultNetwork, false))
	// Configure the network to change the bandwidth, etc.
	t.Run("ConfiguredNetwork", subtestSidecarNetworking(verifyConfiguredNetwork, true))
}
*/

func TestSomething(t *testing.T) {
	hostname, runenv := unique_runenv()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := sdksync.NewInmemClient()
	// Note: Cannot use sdk-go's network.Client. It does not support the InMemory client.
	topic := sdksync.NewTopic("network"+hostname, sdknw.Config{})
	nw := NewMockNetwork()
	t.Log(len(nw.Configured))

	// start sidecar handler.
	inst, err := NewInstance(client, runenv, hostname, nw)
	if err != nil {
		t.Fatal(err)
	}
	go handler(ctx, inst)

	// Now act like a test plan
	client.SignalEntry(ctx, "network-initialized")
	// Request a network change
	// Warning: don't use PublishAndWait. It results in a stack overflow.
	_, err = client.Publish(ctx, topic, &sdknw.Config{})
	if err != nil {
		t.Fatal(err)
	}

	// Let the landler do its work.
	time.Sleep(10 * time.Second)
}
