package sync

// TODO: tests! The code below is copy-pasted from sdk-go.

/*


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
	client, err := redisClient(context.Background(), zap.S())
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
	if client, err = redisClient(context.Background(), zap.S()); err != nil {
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


func TestBarrier(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runenv, cleanup := runtime.RandomTestRunEnv(t)
	t.Cleanup(cleanup)
	defer runenv.Close()

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

	runenv, cleanup := runtime.RandomTestRunEnv(t)
	t.Cleanup(cleanup)
	defer runenv.Close()

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

func TestBarrierZero(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runenv, cleanup := runtime.RandomTestRunEnv(t)
	t.Cleanup(cleanup)
	defer runenv.Close()

	client, err := NewBoundClient(ctx, runenv)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	state := State("apollo")
	ch := client.MustBarrier(ctx, state, 0).C

	select {
	case err := <-ch:
		if err != nil {
			t.Errorf("expected nil error, instead got: %s", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("expected test to finish")
		return
	}
}

func TestBarrierCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runenv, cleanup := runtime.RandomTestRunEnv(t)
	t.Cleanup(cleanup)
	defer runenv.Close()

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

	runenv, cleanup := runtime.RandomTestRunEnv(t)
	t.Cleanup(cleanup)
	defer runenv.Close()

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

func TestInmemSignalAndWait(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runenv, cleanup := runtime.RandomTestRunEnv(t)
	t.Cleanup(cleanup)
	defer runenv.Close()

	client := NewInmemClient()
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
func TestSignalAndWait(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runenv, cleanup := runtime.RandomTestRunEnv(t)
	t.Cleanup(cleanup)
	defer runenv.Close()

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
	runenv, cleanup := runtime.RandomTestRunEnv(t)
	t.Cleanup(cleanup)
	defer runenv.Close()

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

func TestGC(t *testing.T) {
	GCLastAccessThreshold = 1 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	runenv, cleanup := runtime.RandomTestRunEnv(t)
	t.Cleanup(cleanup)
	defer runenv.Close()

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



type TestPayload struct {
	FieldA string
	FieldB struct {
		FieldB1 string
		FieldB2 int
	}
}

func TestSubscribeAfterAllPublished(t *testing.T) {
	var (
		iterations      = 1000
		runenv, cleanup = runtime.RandomTestRunEnv(t)
	)

	t.Cleanup(cleanup)
	defer runenv.Close()

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
		iterations      = 1000
		runenv, cleanup = runtime.RandomTestRunEnv(t)
	)

	t.Cleanup(cleanup)
	defer runenv.Close()

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
		topics          = 100
		iterations      = 100
		runenv, cleanup = runtime.RandomTestRunEnv(t)
	)

	t.Cleanup(cleanup)
	defer runenv.Close()

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
	runenv, cleanup := runtime.RandomTestRunEnv(t)
	t.Cleanup(cleanup)
	defer runenv.Close()

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

	ch2 := make(chan struct{})
	_, err = client.Subscribe(ctx, topic, ch2)
	if err == nil {
		t.Fatalf("expected non-nil error with incorrectly typed channel; got no error")
	}
}

func TestSequenceOnWrite(t *testing.T) {
	var (
		iterations      = 1000
		topic           = &Topic{name: "pandemic", typ: reflect.TypeOf("")}
		runenv, cleanup = runtime.RandomTestRunEnv(t)
	)

	t.Cleanup(cleanup)
	defer runenv.Close()

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

func produce(t *testing.T, client *DefaultClient, topic *Topic, values []TestPayload, pointer bool) {
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



 */