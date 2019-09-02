package sync

import (
	"encoding/json"
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
