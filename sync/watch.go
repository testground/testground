package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
	"sync"

	"github.com/ipfs/testground/logging"

	capi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"
	"github.com/ipfs/testground/api"
)

// Watcher allows us to watch subtrees below a root, which is typically linked
// with this test case and run.
type Watcher struct {
	lk       sync.RWMutex
	client   *capi.Client
	plan     *watch.Plan
	root     string
	subtrees map[*Subtree]map[*typedChan]struct{}
}

type typedChan reflect.Value

// TypedChan wraps a typed channel for use with this watcher. Thank you, Go.
func TypedChan(val interface{}) typedChan {
	v := reflect.ValueOf(val)
	if k := v.Kind(); k != reflect.Chan {
		panic("value is not a channel")
	}
	return typedChan(v)
}

// WatchTree begins watching the subtree underneath this path.
func WatchTree(client *capi.Client, runenv *api.RunEnv) (w *Watcher, err error) {
	var (
		plan   *watch.Plan
		prefix = basePrefix(runenv)
	)

	// Create a plan that watches the entire subtree. We'll mux on this base
	// watch to route updates based on their subpath.
	args := map[string]interface{}{
		"type":   "keyprefix",
		"prefix": prefix,
	}
	if plan, err = watch.Parse(args); err != nil {
		return nil, err
	}

	w = &Watcher{
		client:   client,
		plan:     plan,
		root:     prefix,
		subtrees: make(map[*Subtree]map[*typedChan]struct{}),
	}
	plan.Handler = w.route

	log := log.New(os.Stdout, "watch", log.LstdFlags)
	go plan.RunWithClientAndLogger(client, log)

	return w, nil
}

// Subscribe watches a subtree and emits updates on the specified channel.
func (w *Watcher) Subscribe(subtree *Subtree, ch typedChan) (cancel func(), err error) {
	typ := reflect.Value(ch).Type().Elem()
	subtree.AssertType(typ)

	w.lk.Lock()
	defer w.lk.Unlock()

	// Make sure we have a subtree mapping.
	if _, ok := w.subtrees[subtree]; !ok {
		w.subtrees[subtree] = make(map[*typedChan]struct{}, 2)
	}
	w.subtrees[subtree][&ch] = struct{}{}

	cancel = func() {
		w.lk.Lock()
		defer w.lk.Unlock()
		delete(w.subtrees[subtree], &ch)
	}
	return cancel, nil
}

// Barrier awaits until the specified amount of subtree nodes have been posted
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

// route handles an incoming update, matches it against a subtree, transforms
// the payload, and informs subscribers.
func (w *Watcher) route(index uint64, v interface{}) {
	kvs, ok := v.(capi.KVPairs)
	if !ok {
		logging.S().Warn("watcher received unexpected type")
		return
	}

	w.lk.RLock()
	defer w.lk.RUnlock()

	// For each key value in the notification.
	for _, kv := range kvs {
		// Check the kv against the subtrees we're watching.
		for st, chs := range w.subtrees {
			if !st.MatchFunc(kv) {
				continue
			}

			// If this kv matches this subtree, process it.
			// Deserialize its json into a struct of its type.
			payload := reflect.New(st.PayloadType.Elem())
			if err := json.Unmarshal(kv.Value, payload.Interface()); err != nil {
				logging.S().Warnw("failed to decode value", "data", string(kv.Value), "type", st.PayloadType)
				continue
			}
			for ch := range chs {
				v := reflect.Value(*ch)
				v.Send(payload)
			}
		}
	}
}

// Close closes this watcher. After calling this method, the watcher can't be
// resused.
func (w *Watcher) Close() error {
	w.lk.Lock()
	defer w.lk.Unlock()

	w.plan.Stop()
	w.plan = nil

	for _, chs := range w.subtrees {
		for ch := range chs {
			reflect.Value(*ch).Close()
		}
	}
	w.subtrees = nil
	return nil
}
