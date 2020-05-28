package sidecar

import (
	"context"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/testground/sdk-go/network"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Configures the default network
func TestNetworkInitialize(t *testing.T) {
	reactor, err := NewMockReactor()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	r := reactor.(*MockReactor)

	go func() {
		if err := r.Handle(ctx, handler); err != nil {
			t.Error(err)
		}
	}()

	// Now act like a test plan
	netclient := network.NewClient(r.Client, r.RunEnv)
	netclient.MustWaitNetworkInitialized(ctx)
	assert.Len(t, r.Network.Configured, 1, "the network should be configured once for init")
}

// Test that passing a misconfigured network config throws an appropriate error
func TestNetworkConfiguredFailsMisconfigured(t *testing.T) {
	reactor, err := NewMockReactor()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	r := reactor.(*MockReactor)

	go func() {
		if err := r.Handle(ctx, handler); err != nil {
			t.Error(err)
		}
	}()

	// Now act like a test plan
	netclient := network.NewClient(r.Client, r.RunEnv)
	netclient.MustWaitNetworkInitialized(ctx)
	cfg := network.Config{}
	err = netclient.ConfigureNetwork(ctx, &cfg)
	assert.EqualError(t, err, "failed to configure network; no callback state provided", "the sidecar should not permit invalid network configs")
}

// Test that passing a well-formed configuration succeeds.
func TestNetworkConfigured(t *testing.T) {
	reactor, err := NewMockReactor()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	r := reactor.(*MockReactor)

	go func() {
		if err := r.Handle(ctx, handler); err != nil {
			t.Error(err)
		}
	}()

	// Now act like a test plan
	netclient := network.NewClient(r.Client, r.RunEnv)
	netclient.MustWaitNetworkInitialized(ctx)
	cfg := network.Config{
		Network:       "default",
		Enable:        true,
		CallbackState: "reconfigured",
		Default: network.LinkShape{
			Latency: time.Hour,
		},
	}
	if err = netclient.ConfigureNetwork(ctx, &cfg); err != nil {
		t.Fatal(err)
	}
	assert.Len(t, r.Network.Configured, 2, "the sidecar passes on configurations to the backing network")
	assert.True(t, reflect.DeepEqual(*r.Network.Active["default"], cfg), "the sidecar shuold not edit the config")
}
