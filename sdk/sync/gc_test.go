package sync

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

func TestGC(t *testing.T) {
	GCLastAccessThreshold = 1 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	closeFn := ensureRedis(t)
	defer closeFn()

	runenv := randomRunEnv()

	client, err := NewBoundClient(ctx, runenv)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	b := make([]byte, 16)
	_, _ = rand.Read(b)
	prefix := hex.EncodeToString(b)

	// Create 200 keys.
	for i := 1; i <= 200; i++ {
		state := State(fmt.Sprintf("%s-%d", prefix, i))
		if _, err := client.SignalEntry(ctx, state); err != nil {
			t.Fatal(err)
		}
	}

	pattern := "*" + prefix + "*"
	keys, _ := client.rclient.Keys(pattern).Result()
	if l := len(keys); l != 200 {
		t.Fatalf("expected 200 keys matching %s; got: %d", pattern, l)
	}

	time.Sleep(2 * time.Second)

	ch := make(chan error, 1)
	client.EnableBackgroundGC(ch)

	<-ch

	keys, _ = client.rclient.Keys(pattern).Result()
	if l := len(keys); l != 0 {
		t.Fatalf("expected 0 keys matching %s; got: %d", pattern, l)
	}
}
