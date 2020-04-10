package sync

import (
	"context"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
)

func TestBarrier(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	closeFn := ensureRedis(t)
	defer closeFn()

	runenv := randomRunEnv()

	client, err := NewBoundClient(ctx, runenv)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	state := State("yoda")
	b, err := client.Barrier(ctx, state, 10)
	if err != nil {
		t.Fatal(err)
	}
	ch := b.C

	for i := 1; i <= 10; i++ {
		if curr, err := client.SignalEntry(ctx, state); err != nil {
			t.Fatal(err)
		} else if curr != int64(i) {
			t.Fatalf("expected current count to be: %d; was: %d", i, curr)
		}
	}

	if err := <-ch; err != nil {
		t.Fatal(err)
	}
}

func TestBarrierBeyondTarget(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	closeFn := ensureRedis(t)
	defer closeFn()

	runenv := randomRunEnv()

	client, err := NewBoundClient(ctx, runenv)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	state := State("yoda")
	for i := 1; i <= 20; i++ {
		if curr, err := client.SignalEntry(ctx, state); err != nil {
			t.Fatal(err)
		} else if curr != int64(i) {
			t.Fatalf("expected current count to be: %d; was: %d", i, curr)
		}
	}

	ch := client.MustBarrier(ctx, state, 10).C
	if err := <-ch; err != nil {
		t.Fatal(err)
	}
}

func TestBarrierCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	closeFn := ensureRedis(t)
	defer closeFn()

	runenv := randomRunEnv()

	client, err := NewBoundClient(ctx, runenv)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	state := State("yoda")
	ch := client.MustBarrier(ctx, state, 10).C
	cancel()

	select {
	case err := <-ch:
		if err != context.Canceled {
			t.Errorf("expected context cancelled error; instead got: %s", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("expected a cancel")
		return
	}
}

func TestBarrierDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	closeFn := ensureRedis(t)
	defer closeFn()

	runenv := randomRunEnv()

	client, err := NewBoundClient(ctx, runenv)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	state := State("yoda")
	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	ch := client.MustBarrier(ctx, state, 10).C

	select {
	case err := <-ch:
		if err != context.DeadlineExceeded {
			t.Errorf("expected deadline exceeded error; instead got: %s", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("expected a cancel")
		return
	}
}

func TestSignalAndWait(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	closeFn := ensureRedis(t)
	defer closeFn()

	runenv := randomRunEnv()

	client, err := NewBoundClient(ctx, runenv)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	grp, ctx := errgroup.WithContext(ctx)
	for i := 0; i < 10; i++ {
		grp.Go(func() error {
			_, err := client.SignalAndWait(ctx, State("amber"), 10)
			return err
		})
	}

	if err := grp.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestSignalAndWaitTimeout(t *testing.T) {
	closeFn := ensureRedis(t)
	defer closeFn()

	runenv := randomRunEnv()

	client, err := NewBoundClient(context.Background(), runenv)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	grp, ctx := errgroup.WithContext(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// launch only 9 instead of 10.
	for i := 0; i < 9; i++ {
		grp.Go(func() error {
			_, err := client.SignalAndWait(ctx, State("amber"), 10)
			return err
		})
	}

	if err := grp.Wait(); err != context.DeadlineExceeded {
		t.Fatalf("expected context deadline exceeded error; got: %s", err)
	}
}
