package sync

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/ipfs/testground/api"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/config"
	capi "github.com/hashicorp/consul/api"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

// TODO: start consul on random ports for all endpoints.
func mustStartConsul(t *testing.T) func() {
	t.Helper()

	devMode := true
	builder, err := config.NewBuilder(config.Flags{
		DevMode: &devMode,
	})
	if err != nil {
		t.Fatal(err)
	}
	rt, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}

	log := log.New(os.Stdout, "test", log.LstdFlags)
	ag, err := agent.New(&rt, log)
	if err != nil {
		t.Fatal(err)
	}

	if err := ag.Start(); err != nil {
		t.Fatal(err)
	}

	return func() {
		ag.ShutdownAgent()
		ag.ShutdownEndpoints()
	}
}

func TestWatchWrite(t *testing.T) {
	close := mustStartConsul(t)
	defer close()

	client, err := capi.NewClient(capi.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	runenv := api.RandomRunEnv()

	watcher, err := WatchTree(client, runenv)
	if err != nil {
		t.Fatal(err)
	}

	peersCh := make(chan *peer.AddrInfo, 16)
	cancel, err := watcher.Subscribe(PeerSubtree, TypedChan(peersCh))
	defer cancel()
	if err != nil {
		t.Fatal(err)
	}

	writer, err := NewWriter(client, runenv)
	if err != nil {
		t.Fatal(err)
	}

	ma, err := multiaddr.NewMultiaddr("/ip4/1.2.3.4/tcp/8001/p2p/QmeiLa9HDf5B47utrZHQ1TLcotvCyk2AeVqJrMGRpH5zLu")
	if err != nil {
		t.Fatal(err)
	}

	ai, err := peer.AddrInfoFromP2pAddr(ma)
	if err != nil {
		t.Fatal(err)
	}

	writer.Write(PeerSubtree, ai)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case ai = <-peersCh:
		fmt.Println(ai)
	case <-time.After(5 * time.Second):
		t.Fatal("no event received within 5 seconds")
	}

}
