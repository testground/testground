package sync

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"golang.org/x/sync/errgroup"
)

type TestPayload struct {
	FieldA string
	FieldB struct {
		FieldB1 string
		FieldB2 int
	}
}

func TestSubscribeAfterAllPublished(t *testing.T) {
	var (
		iterations = 1000
		runenv     = randomRunEnv()
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := MustBoundClient(ctx, runenv)
	defer client.Close()

	values := generateTestValues(iterations)

	run := func(pointer bool) func(*testing.T) {
		return func(t *testing.T) {
			topic := &Topic{name: fmt.Sprintf("pandemic:%v", pointer)}
			if pointer {
				topic.typ = reflect.TypeOf(&TestPayload{})
			} else {
				topic.typ = reflect.TypeOf(TestPayload{})
			}

			produce(t, client, topic, values, pointer)

			var ch interface{}
			if pointer {
				ch = make(chan *TestPayload, 128)
			} else {
				ch = make(chan TestPayload, 128)
			}

			_, err := client.Subscribe(ctx, topic, ch)
			if err != nil {
				t.Fatal(err)
			}

			consumeOrdered(t, ctx, ch, values)
		}
	}

	t.Run("pointer type", run(true))
	t.Run("value type", run(false))
}

func TestSubscribeFirstConcurrentWrites(t *testing.T) {
	var (
		iterations = 1000
		runenv     = randomRunEnv()
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := MustBoundClient(ctx, runenv)
	defer client.Close()

	topic := &Topic{name: "virus", typ: reflect.TypeOf("")}

	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan string, 120)
	_ = client.MustSubscribe(ctx, topic, ch)

	grp, ctx := errgroup.WithContext(context.Background())
	v := make([]bool, iterations)
	for i := 0; i < iterations; i++ {
		grp.Go(func() error {
			seq, err := client.Publish(ctx, topic, "foo")
			if err != nil {
				return err
			}
			v[seq-1] = true
			return nil
		})
	}

	if err := grp.Wait(); err != nil {
		t.Fatal(err)
	}

	for i, b := range v {
		if !b {
			t.Fatalf("sequence number absent: %d", i+1)
		}
	}

	// receive all items.
	for i := 0; i < iterations; i++ {
		<-ch
	}

	// no more items queued
	if l := len(ch); l > 0 {
		t.Fatalf("expected no more items queued; got: %d", l)
	}
}

func TestSubscriptionConcurrentPublishersSubscribers(t *testing.T) {
	var (
		topics     = 100
		iterations = 100
		runenv     = randomRunEnv()
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := MustBoundClient(ctx, runenv)
	defer client.Close()

	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	pgrp, ctx := errgroup.WithContext(context.Background())
	sgrp, ctx := errgroup.WithContext(context.Background())
	for i := 0; i < topics; i++ {
		topic := &Topic{name: fmt.Sprintf("antigen-%d", i), typ: reflect.TypeOf("")}

		// launch producer.
		pgrp.Go(func() error {
			for i := 0; i < iterations; i++ {
				_, err := client.Publish(ctx, topic, "foo")
				if err != nil {
					return err
				}
			}
			return nil
		})

		// launch subscriber.
		sgrp.Go(func() error {
			ch := make(chan string, 120)
			_ = client.MustSubscribe(ctx, topic, ch)
			for i := 0; i < iterations; i++ {
				<-ch
			}
			return nil
		})
	}

	if err := pgrp.Wait(); err != nil {
		t.Fatalf("producers failed: %s", err)
	}

	if err := sgrp.Wait(); err != nil {
		t.Fatalf("subscribers failed: %s", err)
	}
}

func TestSubscriptionValidation(t *testing.T) {
	runenv := randomRunEnv()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := MustBoundClient(ctx, runenv)
	defer client.Close()

	topic := &Topic{name: "immune", typ: reflect.TypeOf("")}

	ctx, cancel = context.WithCancel(context.Background())

	ch := make(chan string, 1)
	_, err := client.Subscribe(ctx, topic, ch)
	if err != nil {
		t.Fatalf("expected nil error when adding subscription; got: %s", err)
	}
	_, err = client.Subscribe(ctx, topic, ch)
	if err == nil {
		t.Fatalf("expected non-nil error with duplicate subscription; got no error")
	}

	cancel()

	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	ch = make(chan string)
	_, err = client.Subscribe(ctx, topic, ch)
	if err == nil {
		t.Fatalf("expected non-nil error with bufferless channel; got no error")
	}

	ch2 := make(chan struct{})
	_, err = client.Subscribe(ctx, topic, ch2)
	if err == nil {
		t.Fatalf("expected non-nil error with incorrectly typed channel; got no error")
	}
}

func TestSequenceOnWrite(t *testing.T) {
	var (
		iterations = 1000
		runenv     = randomRunEnv()
		topic      = &Topic{name: "pandemic", typ: reflect.TypeOf("")}
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := MustBoundClient(ctx, runenv)
	defer client.Close()

	s := "a"
	for i := 1; i <= iterations; i++ {
		seq, err := client.Publish(ctx, topic, s)
		if err != nil {
			t.Fatal(err)
		}

		if seq != int64(i) {
			t.Fatalf("expected seq %d, got %d", i, seq)
		}
	}
}

func consumeOrdered(t *testing.T, ctx context.Context, ch interface{}, values []TestPayload) {
	t.Helper()

	for i, expected := range values {
		chosen, recv, _ := reflect.Select([]reflect.SelectCase{
			{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)},
			{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ctx.Done())},
		})

		switch chosen {
		case 0:
			if recv.Kind() == reflect.Ptr {
				recv = recv.Elem()
			}
			if recv.Interface() != reflect.ValueOf(expected).Interface() {
				t.Fatalf("expected value %v, got %v in position %d", expected, recv, i)
			}
		case 1:
			t.Fatal("failed to receive all expected items within the deadline")
		}
	}
}

func produce(t *testing.T, client *Client, topic *Topic, values []TestPayload, pointer bool) {
	for i, s := range values {
		var v interface{}
		if pointer {
			v = &s
		} else {
			v = s
		}

		if seq, err := client.Publish(context.Background(), topic, v); err != nil {
			t.Fatalf("failed while writing key to subtree: %s", err)
		} else if seq != int64(i)+1 {
			t.Fatalf("expected seq == i+1; seq: %d; i: %d", seq, i)
		}
	}
}

func generateTestValues(length int) []TestPayload {
	values := make([]TestPayload, 0, length)
	for i := 0; i < length; i++ {
		s := fmt.Sprintf("item-%d", i)
		values = append(values, TestPayload{
			FieldA: s,
			FieldB: struct {
				FieldB1 string
				FieldB2 int
			}{FieldB1: s, FieldB2: i},
		})
	}
	return values
}
