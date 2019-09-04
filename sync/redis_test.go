package sync

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/go-redis/redis"
	"github.com/ipfs/testground/api"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/test"
	"github.com/multiformats/go-multiaddr"
)

// Check if there's a running instance of redis, or start it otherwise.
func ensureRedis(t *testing.T) (client *redis.Client, close func()) {
	t.Helper()

	// Try to obtain a client; if this fails, attempt to start a redis instance.
	client, err := RedisClient()
	if err == nil {
		return client, func() {}
	}

	cmd := exec.Command("redis-server", "-")
	// enable keyspace events.
	cmd.Stdin = strings.NewReader(`notify-keyspace-events "$szxKE"`)
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start redis: %s", err)
	}

	time.Sleep(1 * time.Second)

	// Try to obtain a client again.
	if client, err = RedisClient(); err != nil {
		t.Fatalf("failed to obtain redis client despite starting instance: %v", err)
	}

	return client, func() {
		if err := cmd.Process.Kill(); err != nil {
			t.Fatalf("failed while stopping test-scoped redis: %s", err)
		}
	}
}

func TestWatcherWriter(t *testing.T) {
	client, close := ensureRedis(t)
	defer close()

	runenv := api.RandomRunEnv()

	watcher, err := NewWatcher(client, runenv)
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

func TestBarrier(t *testing.T) {
	client, close := ensureRedis(t)
	defer close()

	runenv := api.RandomRunEnv()

	watcher, writer := MustWatcherWriter(client, runenv)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := watcher.Barrier(ctx, PeerSubtree, 10)
	if err != nil {
		t.Fatal(err)
	}

	publishPeer := func() {
		id := test.RandPeerIDFatal(t)
		ma, err := multiaddr.NewMultiaddr("/ip4/1.2.3.4/tcp/8001/p2p/" + id.Pretty())
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
	}

	for i := 0; i < 10; i++ {
		publishPeer()
	}

	if err := <-ch; err != nil {
		t.Fatal(err)
	}
}
