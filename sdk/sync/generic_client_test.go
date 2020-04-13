package sync

import (
	"context"
	"reflect"
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestGenericClientRunEnv checks that states and payloads published by a bound
// client can be seen by a generic client when using the right RunParams, and
// vice versa.
func TestGenericClientRunEnv(t *testing.T) {
	var (
		state1 = State("state1")
		state2 = State("state2")

		// passing in the type token.
		topic1 = NewTopic("topic1", reflect.TypeOf(""))
		// letting the constructor derive the type token.
		topic2 = NewTopic("topic2", "")
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	runenv := randomRunEnv()

	bclient, err := NewBoundClient(ctx, runenv)
	if err != nil {
		t.Fatal(err)
	}
	defer bclient.Close()

	gclient, err := NewGenericClient(ctx, zap.S())
	if err != nil {
		t.Fatal(err)
	}
	defer gclient.Close()

	// Generic barrier || Bound signals
	ch := gclient.MustBarrier(WithRunParams(ctx, &runenv.RunParams), state1, 1).C
	_ = bclient.MustSignalEntry(ctx, state1)
	select {
	case err := <-ch:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("barrier failed to trigger within 5 seconds")
	}

	// Bound barrier || Generic signals
	ch = bclient.MustBarrier(ctx, state2, 1).C
	_ = gclient.MustSignalEntry(WithRunParams(ctx, &runenv.RunParams), state2)
	select {
	case err := <-ch:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("barrier failed to trigger within 5 seconds")
	}

	// Generic publishes || Bound subscribes.
	sch := make(chan string, 10)
	bclient.MustSubscribe(ctx, topic1, sch)
	for i := 0; i < 10; i++ {
		_ = gclient.MustPublish(WithRunParams(ctx, &runenv.RunParams), topic1, "foo")
		<-sch
	}

	// Bound publishes || Generic subscribes.
	sch = make(chan string, 10)
	gclient.MustSubscribe(WithRunParams(ctx, &runenv.RunParams), topic2, sch)
	for i := 0; i < 10; i++ {
		_ = bclient.MustPublish(ctx, topic2, "foo")
		<-sch
	}
}

func TestGenericClientRequiresRunParams(t *testing.T) {
	gclient, err := NewGenericClient(context.Background(), zap.S())
	if err != nil {
		t.Fatal(err)
	}
	defer gclient.Close()

	_, err = gclient.Subscribe(context.Background(), &Topic{}, make(chan string))
	if err != ErrNoRunParameters {
		t.Fatalf("error not ErrNoRunParameters; was: %s", err)
	}

	_, err = gclient.Publish(context.Background(), &Topic{}, "")
	if err != ErrNoRunParameters {
		t.Fatalf("error not ErrNoRunParameters; was: %s", err)
	}

	_, err = gclient.SignalEntry(context.Background(), State("foo"))
	if err != ErrNoRunParameters {
		t.Fatalf("error not ErrNoRunParameters; was: %s", err)
	}

	_, err = gclient.Barrier(context.Background(), State("foo"), 100)
	if err != ErrNoRunParameters {
		t.Fatalf("error not ErrNoRunParameters; was: %s", err)
	}
}
