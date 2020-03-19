package sync

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/ipfs/testground/sdk/runtime"

	"github.com/go-redis/redis/v7"
)

// Watcher exposes methods to watch subtrees within the sync tree of this test.
type Watcher struct {
	re        *runtime.RunEnv
	client    *redis.Client
	root      string
	subs      sync.WaitGroup
	close     chan struct{}
	closeOnce sync.Once
}

// NewWatcher begins watching the subtree underneath this path.
//
// NOTE: Canceling the context cancels the call to this function, it does not
// affect the returned watcher.
func NewWatcher(ctx context.Context, runenv *runtime.RunEnv) (w *Watcher, err error) {
	client, err := getGlobalRedisClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("during redisClient: %w", err)
	}
	return NewWatcherWithClient(ctx, client, runenv)
}

func NewWatcherWithClient(ctx context.Context, client *redis.Client, runenv *runtime.RunEnv) (w *Watcher, err error) {
	prefix := basePrefix(runenv)
	w = &Watcher{
		re:     runenv,
		client: client,
		root:   prefix,
		close:  make(chan struct{}),
	}
	return w, nil
}

// Subscribe watches a subtree and emits updates on the specified channel.
//
// The element type of the channel must match the payload type of the Subtree.
//
// We close the supplied channel when the subscription ends, in all cases. At
// that point, the caller should consume the error (or nil value) from the
// returned errCh.
//
// The user can cancel the subscription by calling the returned cancelFn or by
// canceling the passed context. The subscription will die if an internal error
// occurs.
func (w *Watcher) Subscribe(ctx context.Context, subtree *Subtree, ch interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := w.client.Context().Err(); err != nil {
		return err
	}

	chV := reflect.ValueOf(ch)
	if k := chV.Kind(); k != reflect.Chan {
		return fmt.Errorf("value is not a channel: %T", ch)
	}

	if err := subtree.AssertType(chV.Type().Elem()); err != nil {
		chV.Close()
		return err
	}

	root := w.root + ":" + subtree.GroupKey
	sub := &subscription{
		w:       w,
		subtree: subtree,
		client:  w.client.WithContext(ctx),
		key:     root,
		outCh:   chV,
	}

	// Start the subscription.
	w.subs.Add(1)
	go func() {
		defer w.subs.Done()
		sub.process()
	}()

	return nil
}

// Barrier awaits until the specified amount of items are advertising to be in
// the provided state. It returns a channel on which two things can happen:
//
//   a. if enough items appear before the context fires, a nil
//      error will be sent.
//   b. if the context fires, or another error occurs during the
//      process, an error is propagated in the channel.
//
// In both cases, the chan will only receive a single element before closure.
func (w *Watcher) Barrier(ctx context.Context, state State, required int64) <-chan error {
	log := w.re.SLogger()

	log.Debugw("setting barrier for state", "state", state, "required", required)

	var (
		client      = w.client.WithContext(ctx)
		stateKey    = state.Key(w.root)
		barrierChan = state.BarrierChannel(w.root, required)
		resCh       = make(chan error, 1)
	)

	// Subscribe to receive barrier advisory signals.
	//
	// Each call INCRing the key will also PUBLISH a message to a channel with format:
	//
	//   <root>/states/<state>/barriers/<value>, where <value> is the new value post-increment.
	//
	// Waiters will park waiting to receive a message on their corresponding channel, where <value> is their target.
	ps := client.Subscribe(barrierChan)
	iface, err := ps.Receive()
	if err != nil {
		resCh <- fmt.Errorf("failed to subscribe to barrier channel: %w", err)
		close(resCh)
		return resCh
	}

	// Wait until we receive the subscription confirmation.
	if _, ok := iface.(*redis.Subscription); !ok {
		resCh <- fmt.Errorf("expected first message on barrier subscription to be of type *Subscription; was: %T", iface)
		close(resCh)
		return resCh
	}

	// checkValue is a function that checks the current value of the state, and
	// if we're on a terminal state (error, or satisfied), it publishes the
	// right value on resCh, and returns done=true.
	checkValue := func() (curr int64, done bool) {
		curr, err := client.Get(stateKey).Int64()
		switch {
		case err != nil && err != redis.Nil:
			resCh <- fmt.Errorf("failed to get value of state: %w", err)
			return 0, false
		case curr >= required:
			resCh <- nil
			return curr, true
		}
		return curr, false
	}

	if _, done := checkValue(); done {
		_ = ps.Close()
		close(resCh)
		return resCh
	}

	go func() {
		defer close(resCh)

		select {
		case <-ps.Channel():
			if curr, done := checkValue(); !done {
				resCh <- fmt.Errorf("barrier advisory signal for state %s received, but assertion failed; current value (%d) below target (%d)", state, curr, required)
			}
		case <-ctx.Done():
			resCh <- fmt.Errorf("context closed waiting on %s; required: %d", state, required)

		case <-w.close:
			resCh <- fmt.Errorf("closed")
		}
	}()

	return resCh
}

// Close closes this watcher. After calling this method, the watcher can't be
// resused.
//
// Note: Concurrently closing the watcher while calling Subscribe may panic.
func (w *Watcher) Close() error {
	w.closeOnce.Do(func() {
		close(w.close)
		w.subs.Wait()
	})
	return nil
}
