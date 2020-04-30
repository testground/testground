package sidecar

import (
	"context"
	"github.com/alicebob/miniredis/v2"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
	"math/rand"
	"testing"
	"time"
)

var TST_RUNPARAMS = runtime.RunParams{
	TestPlan:               "sidecarPlan",
	TestCase:               "sidecarCase",
	TestRun:                "123",
	TestInstanceCount:      1,
	TestInstanceRole:       "sidecarRole",
	TestGroupID:            "sidecarGroup",
	TestGroupInstanceCount: 1,
}

func TestHandler(t *testing.T) {
	r, err := miniredis.Run()
	if err != nil {
		t.Error(err)
	}
	defer r.Close()

	runenv := runtime.NewRunEnv(TST_RUNPARAMS)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, err := sync.NewBoundClient(ctx, runenv)
	if err != nil {
		t.Error(err)
	}

	nw := NewMockNetwork()

	time.Sleep(time.Duration(rand.Int()%10) * time.Second)
	rand.Seed(time.Now().UnixNano())
	hostname := "sidecar.test" + string(rand.Int())

	inst, err := NewInstance(client, runenv, hostname, nw)
	if err != nil {
		t.Error(err)
	}
	go handler(ctx, inst)
	_, err = client.SignalEntry(ctx, "network-initialized")
	if err != nil {
		t.Error(err)
	}

	time.Sleep(5 * time.Second) // Wait for the handler to see us and have time to subscribe.

	netCfg := sync.NetworkConfig{
		Network: "mynetwork",
		State:   "mynetworkdone",
	}
	topic := sync.NetworkTopic(inst.Hostname)
	client.MustPublishAndWait(ctx, topic, netCfg, "mynetworkdone", 1)

	// verify that the handler has configured the network
	numConf := len(nw.Configured)
	if numConf != 1 {
		t.Error("Expected to be configured one time. Actual number of configures:", numConf)
		for _, x := range nw.Configured {
			t.Error(x.Network)
		}
	}
}
