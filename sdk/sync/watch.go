package sync

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/ipfs/testground/sdk/runtime"

	"github.com/go-redis/redis"
	"github.com/hashicorp/go-multierror"
)

// Watcher exposes methods to watch subtrees within the sync tree of this test.
type Watcher struct {
	lk       sync.RWMutex
	re       *runtime.RunEnv
	client   *redis.Client
	root     string
	subtrees map[*Subtree]map[*subscription]struct{}
}

// NewWatcher begins watching the subtree underneath this path.
func NewWatcher(runenv *runtime.RunEnv) (w *Watcher, err error) {
	client, err := redisClient(runenv)
	if err != nil {
		return nil, err
	}

	prefix := basePrefix(runenv)
	w = &Watcher{
		re:       runenv,
		client:   client,
		root:     prefix,
		subtrees: make(map[*Subtree]map[*subscription]struct{}),
	}
	return w, nil
}

// Subscribe watches a subtree and emits updates on the specified channel.
//
// The element type of the channel must match the payload type of the Subtree.
func (w *Watcher) Subscribe(subtree *Subtree, ch interface{}) (cancel func() error, err error) {
	chV := reflect.ValueOf(ch)
	if k := chV.Kind(); k != reflect.Chan {
		return nil, fmt.Errorf("value is not a channel: %T", ch)
	}
	if err = subtree.AssertType(chV.Type().Elem()); err != nil {
		return nil, err
	}

	w.lk.Lock()

	// Make sure we have a subtree mapping.
	if _, ok := w.subtrees[subtree]; !ok {
		w.subtrees[subtree] = make(map[*subscription]struct{})
	}

	root := w.root + ":" + subtree.GroupKey
	sub := &subscription{
		w:       w,
		subtree: subtree,
		client:  w.client,
		root:    root,
		outCh:   chV,
	}

	w.subtrees[subtree][sub] = struct{}{}
	w.lk.Unlock()

	// Start the subscription.
	if err := sub.start(); err != nil {
		return nil, err
	}

	cancel = func() error {
		w.lk.Lock()
		defer w.lk.Unlock()

		delete(w.subtrees[subtree], sub)
		if len(w.subtrees[subtree]) == 0 {
			delete(w.subtrees, subtree)
		}

		return sub.stop()
	}
	return cancel, nil
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
	resCh := make(chan error)

	go func() {
		defer close(resCh)

		var (
			last   int64
			err    error
			ticker = time.NewTicker(250 * time.Millisecond)
			k      = state.Key(w.root)
		)

		defer ticker.Stop()

		for last != required {
			select {
			case <-ticker.C:
				last, err = w.client.Get(k).Int64()
				if err != nil && err != redis.Nil {
					err = fmt.Errorf("error occured in barrier: %w", err)
					resCh <- err
					return
				}
				// loop over
			case <-ctx.Done():
				// Context fired before we got enough elements.
				err := fmt.Errorf("context deadline exceeded; not enough elements, required: %d, got: %d", required, last)
				resCh <- err
				return
			}
		}
		resCh <- nil
	}()

	return resCh
}

// Close closes this watcher. After calling this method, the watcher can't be
// resused.
func (w *Watcher) Close() error {
	w.lk.Lock()
	defer w.lk.Unlock()

	var result *multierror.Error
	for _, st := range w.subtrees {
		for sub := range st {
			result = multierror.Append(result, sub.stop())
		}
	}
	w.subtrees = nil
	return result.ErrorOrNil()
}
