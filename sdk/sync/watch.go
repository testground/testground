package sync

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/ipfs/testground/sdk/runtime"

	"github.com/go-redis/redis"
	"github.com/hashicorp/go-multierror"
)

type typedChan reflect.Value

// TypedChan wraps a typed channel for use with this watcher. Thank you, Go.
func TypedChan(val interface{}) typedChan {
	v := reflect.ValueOf(val)
	if k := v.Kind(); k != reflect.Chan {
		panic("value is not a channel")
	}
	return typedChan(v)
}

// Watcher exposes methods to watch subtrees within the sync tree of this test.
type Watcher struct {
	lk       sync.RWMutex
	re       *runtime.RunEnv
	client   *redis.Client
	root     string
	subtrees map[*Subtree]map[*subscription]struct{}
}

// NewWatcher begins watching the subtree underneath this path.
func NewWatcher(client *redis.Client, runenv *runtime.RunEnv) (w *Watcher, err error) {
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
// Wrap the channel in the TypedChan() function before passing it into this
// method.
func (w *Watcher) Subscribe(subtree *Subtree, ch typedChan) (cancel func() error, err error) {
	typ := reflect.Value(ch).Type().Elem()
	if err = subtree.AssertType(typ); err != nil {
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
		outCh:   ch,
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

// Barrier awaits until the specified amount of subtree items have been posted
// on the subtree. It returns a channel on which two things can happen:
//
//   a. if enough subtree nodes were written before the context fired, a nil
//      error will be sent.
//   b. if the context was exceeded, or another error occurred during the
//      subscription, an error will be propagated.
//
// In both cases, the chan will only receive a single element before closure.
func (w *Watcher) Barrier(ctx context.Context, subtree *Subtree, count int) (<-chan error, error) {
	chTyp := reflect.ChanOf(reflect.BothDir, subtree.PayloadType)
	subCh := reflect.MakeChan(chTyp, 0)
	resCh := make(chan error)

	// No need to take the lock here, as Subscribe does.
	cancelSub, err := w.Subscribe(subtree, TypedChan(subCh.Interface()))
	if err != nil {
		return nil, err
	}

	// Pump values to an untyped channel, so we can consume in the select block
	// below.
	next := make(chan interface{})
	go func() {
		for {
			val, ok := subCh.Recv()
			if !ok {
				close(next)
				return
			}
			next <- val.Interface()
		}
	}()

	// This goroutine drains the subscription channel and increments a counter.
	// When we receive enough elements, we finish and inform the caller. If the
	// context fires, or an error occurs, we inform the caller of the error
	// condition.
	go func() {
		defer close(resCh)
		defer cancelSub()

		for rcvd := 0; rcvd < count; rcvd++ {
			select {
			case _, ok := <-next:
				if !ok {
					// Subscription channel was closed early.
					resCh <- fmt.Errorf("subscription closed early; not enough elements, requested: %d, got: %d", count, rcvd)
					return
				}
				continue
			case <-ctx.Done():
				// Context fired before we got enough elements.
				resCh <- fmt.Errorf("context deadline exceeded; not enough elements, requested: %d, got: %d", count, rcvd)
				return
			}
		}
		// We got as many elements as required.
		resCh <- nil
	}()

	return resCh, nil
}

// Close closes this watcher. After calling this method, the watcher can't be
// resused.
func (w *Watcher) Close() error {
	w.lk.Lock()
	defer w.lk.Unlock()

	var result *multierror.Error
	for _, st := range w.subtrees {
		for sub := range st {
			multierror.Append(result, sub.stop())
		}
	}
	w.subtrees = nil
	return result.ErrorOrNil()
}
