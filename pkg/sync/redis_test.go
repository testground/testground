package sync

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v7"
	"github.com/google/uuid"
	"github.com/testground/testground/pkg/logging"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"math/rand"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	// Avoid collisions in Redis keys over test runs.
	rand.Seed(time.Now().UnixNano())

	// _ = os.Setenv("LOG_LEVEL", "debug")

	// Set fail-fast options for creating the client, capturing the default
	// state to restore it.
	prev := DefaultRedisOpts
	DefaultRedisOpts.PoolTimeout = 500 * time.Millisecond
	DefaultRedisOpts.MaxRetries = 0

	closeFn, err := ensureRedis()
	DefaultRedisOpts = prev
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	v := m.Run()

	_ = closeFn()
	os.Exit(v)
}

// Check if there's a running instance of redis, or start it otherwise. If we
// start an ad-hoc instance, the close function will terminate it.
func ensureRedis() (func() error, error) {
	// Try to obtain a client; if this fails, we'll attempt to start a redis
	// instance.
	client, err := redisClient(context.Background(), zap.S(), &RedisConfiguration{
		Host: "localhost",
	})
	if err == nil {
		_ = client.Close()
		return func() error { return nil }, err
	}

	cmd := exec.Command("redis-server", "-")
	if err := cmd.Start(); err != nil {
		return func() error { return nil }, fmt.Errorf("failed to start redis: %w", err)
	}

	time.Sleep(1 * time.Second)

	// Try to obtain a client again.
	if client, err = redisClient(context.Background(), zap.S(), &RedisConfiguration{
		Host: "localhost",
	}); err != nil {
		return func() error { return nil }, fmt.Errorf("failed to obtain redis client despite starting instance: %v", err)
	}
	defer client.Close()

	return func() error {
		if err := cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed while stopping test-scoped redis: %s", err)
		}
		return nil
	}, nil
}

func getService(ctx context.Context) (*RedisService, error) {
	return NewRedisService(ctx, logging.S(), &RedisConfiguration{
		Port: 6379,
		Host: "localhost",
	})
}

func TestBarrier(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	service, err := getService(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	state := "yoda"
	go func() {
		for i := 1; i <= 10; i++ {
			if curr, err := service.SignalEntry(ctx, state); err != nil {
				t.Fatal(err)
			} else if curr != int64(i) {
				t.Fatalf("expected current count to be: %d; was: %d", i, curr)
			}
		}

	}()

	err = service.Barrier(ctx, state, 10)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBarrierBeyondTarget(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	service, err := getService(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	state := "yoda"
	for i := 1; i <= 20; i++ {
		if curr, err := service.SignalEntry(ctx, state); err != nil {
			t.Fatal(err)
		} else if curr != int64(i) {
			t.Fatalf("expected current count to be: %d; was: %d", i, curr)
		}
	}

	err = service.Barrier(ctx, state, 10)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBarrierZero(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	service, err := getService(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	go func() {
		select {
		case <-time.After(3 * time.Second):
			t.Error("expected test to finish")
		}
	}()

	err = service.Barrier(ctx, "apollo", 0)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBarrierCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	service, err := getService(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	go func() {
		select {
		case <-time.After(1 * time.Second):
			cancel()
		case <-time.After(3 * time.Second):
			t.Error("expected a cancel")
		}
	}()

	err = service.Barrier(ctx, "yoda", 10)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context cancelled error; instead got: %s", err)
	}
}

func TestBarrierDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	service, err := getService(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	go func() {
		select {
		case <-time.After(3 * time.Second):
			t.Error("expected a cancel")
		}
	}()

	err = service.Barrier(ctx, "yoda", 10)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context cancelled error; instead got: %s", err)
	}
}

func TestGC(t *testing.T) {
	GCLastAccessThreshold = 1 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	service, err := getService(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	b := make([]byte, 16)
	_, _ = rand.Read(b)
	prefix := hex.EncodeToString(b)

	// Create 200 keys.
	for i := 1; i <= 200; i++ {
		state := fmt.Sprintf("%s-%d", prefix, i)
		if _, err := service.SignalEntry(ctx, state); err != nil {
			t.Fatal(err)
		}
	}

	pattern := "*" + prefix + "*"
	keys, _ := service.rclient.Keys(pattern).Result()
	if l := len(keys); l != 200 {
		t.Fatalf("expected 200 keys matching %s; got: %d", pattern, l)
	}

	time.Sleep(2 * time.Second)

	ch := make(chan error, 1)
	service.EnableBackgroundGC(ch)

	<-ch

	keys, _ = service.rclient.Keys(pattern).Result()
	if l := len(keys); l != 0 {
		t.Fatalf("expected 0 keys matching %s; got: %d", pattern, l)
	}
}

func TestConnUnblock(t *testing.T) {
	client := redis.NewClient(&redis.Options{})
	c := client.Conn()
	id, _ := c.ClientID().Result()

	ch := make(chan struct{})
	go func() {
		timer := time.AfterFunc(1*time.Second, func() { close(ch) })
		_, _ = c.XRead(&redis.XReadArgs{Streams: []string{"aaaa", "0"}, Block: 0}).Result()
		timer.Stop()
	}()

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("XREAD unexpectedly returned early")
	}

	unblocked, err := client.ClientUnblock(id).Result()
	if err != nil {
		t.Fatal(err)
	}
	if unblocked != 1 {
		t.Fatal("expected CLIENT UNBLOCK to return 1")
	}
	for i := 0; i < 10; i++ {
		id2, err := c.ClientID().Result()
		if err != nil {
			t.Fatal(err)
		}
		if id != id2 {
			t.Errorf("expected client id to be: %d, was: %d", id, id2)
		}
	}
}

/*
type TestPayload struct {
	FieldA string
	FieldB struct {
		FieldB1 string
		FieldB2 int
	}
}

func TestSubscribeAfterAllPublished(t *testing.T) {
	iterations := 1000
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service, err := getService(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	values := make([]TestPayload, 0, iterations)
	for i := 0; i < iterations; i++ {
		s := fmt.Sprintf("item-%d", i)
		values = append(values, TestPayload{
			FieldA: s,
			FieldB: struct {
				FieldB1 string
				FieldB2 int
			}{FieldB1: s, FieldB2: i},
		})
	}

	topic := fmt.Sprintf("pandemic:still2021")

	for i, s := range values {
		if seq, err := service.Publish(context.Background(), topic, s); err != nil {
			t.Fatalf("failed while writing key to subtree: %s", err)
		} else if seq != int64(i)+1 {
			t.Fatalf("expected seq == i+1; seq: %d; i: %d", seq, i)
		}
	}

	sub, err := service.Subscribe(ctx, topic)
	if err != nil {
		t.Fatal(err)
	}

	for i, expected := range values {
		chosen, recv, _ := reflect.Select([]reflect.SelectCase{
			{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(sub.outCh)},
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
} */

func TestSubscribeFirstConcurrentWrites(t *testing.T) {
	iterations := 1000
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service, err := getService(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	topic := "virus:" + uuid.New().String()

	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	sub, err := service.Subscribe(ctx, topic)
	if err != nil {
		t.Fatal(err)
	}

	grp, ctx := errgroup.WithContext(context.Background())
	v := make([]bool, iterations)
	for i := 0; i < iterations; i++ {
		grp.Go(func() error {
			seq, err := service.Publish(ctx, topic, "foo")
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
		<-sub.outCh
	}

	// no more items queued
	if l := len(sub.outCh); l > 0 {
		t.Fatalf("expected no more items queued; got: %d", l)
	}
}

func TestSubscriptionConcurrentPublishersSubscribers(t *testing.T) {
	var (
		topics     = 100
		iterations = 100
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service, err := getService(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	pgrp, ctx := errgroup.WithContext(context.Background())
	sgrp, ctx := errgroup.WithContext(context.Background())
	for i := 0; i < topics; i++ {
		topic := fmt.Sprintf("antigen-%d", i)

		// launch producer.
		pgrp.Go(func() error {
			for i := 0; i < iterations; i++ {
				_, err := service.Publish(ctx, topic, "foo")
				if err != nil {
					return err
				}
			}
			return nil
		})

		// launch subscriber.
		sgrp.Go(func() error {
			sub, err := service.Subscribe(ctx, topic)
			if err != nil {
				return err
			}
			for i := 0; i < iterations; i++ {
				<-sub.outCh
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

func TestSequenceOnWrite(t *testing.T) {
	var (
		iterations = 1000
		topic      = "pandemic:" + uuid.New().String()
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service, err := getService(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	s := "a"
	for i := 1; i <= iterations; i++ {
		seq, err := service.Publish(ctx, topic, s)
		if err != nil {
			t.Fatal(err)
		}

		if seq != int64(i) {
			t.Fatalf("expected seq %d, got %d", i, seq)
		}
	}
}

/*

func TestRedisHost(t *testing.T) {
	realRedisHost := os.Getenv(EnvRedisHost)
	defer os.Setenv(EnvRedisHost, realRedisHost)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = os.Setenv(EnvRedisHost, "redis-does-not-exist.example.com")
	client, err := redisClient(ctx, zap.S())
	if err == nil {
		_ = client.Close()
		t.Error("should not have found redis host")
	}

	_ = os.Setenv(EnvRedisHost, "redis-does-not-exist.example.com")
	client, err = redisClient(ctx, zap.S())
	if err == nil {
		_ = client.Close()
		t.Error("should not have found redis host")
	}

	realHost := realRedisHost
	if realHost == "" {
		realHost = "localhost"
	}
	_ = os.Setenv(EnvRedisHost, realHost)
	client, err = redisClient(ctx, zap.S())
	if err != nil {
		t.Errorf("expected to establish connection to redis, but failed with: %s", err)
	}
	_ = client.Close()
}

*/
