package sync

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/ipfs/testground/sdk/runtime"

	"github.com/go-redis/redis/v7"
)

var (
	// TTL is the expiry of the records this writer inserts.
	TTL = 10 * time.Second

	// KeepAlivePeriod is half the TTL. The Writer extends the TTL of the
	// records it owns with this frequency.
	KeepAlivePeriod = TTL / 2
)

// Writer offers an API to write objects to the sync tree for a running test.
//
// The sync service is designed to run in a distributed test system, where
// things will fail and end ungracefully. To avoid an ever-growing population of
// leftover records in Redis, all keys we write have a TTL, as per the sync.TTL
// var. We keep custody of these keys by periodically extending the TTL while
// the test instance is running. See godoc on keepAlive* methods and struct
// fields for more information.
type Writer struct {
	lk     sync.RWMutex
	client *redis.Client
	re     *runtime.RunEnv
	cancel context.CancelFunc

	// root is the namespace under which this test run writes. It is derived
	// from the RunEnv.
	root string

	// keepAliveSet are the keys we are responsible for keeping alive. This is a
	// superset of ownset + special keys we are responsible for keeping alive.
	keepAliveSet map[string]struct{}
}

// NewWriter creates a new Writer for a specific test run, as defined by the
// RunEnv.
//
// NOTE: Canceling the context cancels the call to this function, it does not
// affect the returned watcher.
func NewWriter(ctx context.Context, runenv *runtime.RunEnv) (w *Writer, err error) {
	client, err := redisClient(ctx, runenv)
	if err != nil {
		return nil, err
	}

	exitCtx, cancel := context.WithCancel(context.Background())
	w = &Writer{
		client:       client,
		re:           runenv,
		root:         basePrefix(runenv),
		cancel:       cancel,
		keepAliveSet: make(map[string]struct{}),
	}

	// Start a background worker that keeps alive the keeys
	go w.keepAliveWorker(exitCtx)
	return w, nil
}

// keepAliveWorker runs a loop that extends the TTL in the keepAliveSet every
// `KeepAlivePeriod`. It should be launched as a goroutine.
func (w *Writer) keepAliveWorker(ctx context.Context) {
	for {
		select {
		case <-time.After(KeepAlivePeriod):
			w.keepAlive(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// keepAlive extends the TTL of all keys in the keepAliveSet.
func (w *Writer) keepAlive(ctx context.Context) {
	w.lk.RLock()
	defer w.lk.RUnlock()

	// TODO: do this in a transaction. We risk the loop overlapping with the
	// refresh period, and all kinds of races. We need to be adaptive here.
	for k := range w.keepAliveSet {
		if err := w.client.WithContext(ctx).Expire(k, TTL).Err(); err != nil {
			panic(err)
		}
	}
}

// Write writes a payload in the sync tree for the test, which is backed by a
// Redis stream.
//
// It _panics_ if the payload's type does not match the expected type for the
// subtree.
//
// If the actual write on the sync service fails, this method returns an error.
//
// Else, if all succeeds, it returns the ordinal sequence number of this entry
// within the subtree (starting at 1).
func (w *Writer) Write(ctx context.Context, subtree *Subtree, payload interface{}) (seq int64, err error) {
	if err = subtree.AssertType(reflect.ValueOf(payload).Type()); err != nil {
		return -1, err
	}

	// Serialize the payload.
	bytes, err := json.Marshal(payload)
	if err != nil {
		return -1, err
	}

	// Calculate the stream key.
	key := w.root + ":" + subtree.GroupKey

	// Perform a Redis transaction, adding the item to the stream and fetching
	// the XLEN of the stream.
	var xlen *redis.IntCmd
	_, err = w.client.WithContext(ctx).TxPipelined(func(pipe redis.Pipeliner) error {
		pipe.XAdd(&redis.XAddArgs{
			Stream: key,
			ID:     "*",
			Values: map[string]interface{}{
				RedisStreamPayloadKey: bytes,
			},
		})
		xlen = pipe.XLen(key)
		return nil
	})

	if err != nil {
		return -1, err
	}

	seq = xlen.Val()

	w.re.SLogger().Debugw("wrote payload to stream", "subtree", subtree.GroupKey, "our_seq", seq)

	// If we are within the first 5 nodes writing to this subtree, we're
	// responsible for keeping the stream alive. Having _all_ nodes refreshing
	// the stream keys would be wasteful, so selecting a few supervisors
	// deterministically is appropriate.
	//
	// TODO(raulk) this is not entirely sound. In some test choreographies, the
	// first 5 nodes may legitimately finish first (or they may be biased in
	// some other manner), in which case these keys would cease being refreshed,
	// and they'd expire early. The situation is both improbable, and unlikely
	// to become a problem. Let's cross that bridge when we get to it.
	if seq <= 5 {
		w.lk.Lock()
		w.keepAliveSet[key] = struct{}{}
		w.lk.Unlock()
	}

	return seq, err
}

// SignalEntry signals entry into the specified state, and returns how many
// instances are currently in this state, including the caller.
func (w *Writer) SignalEntry(ctx context.Context, s State) (current int64, err error) {
	log := w.re.SLogger()

	log.Debugw("signalling entry to state", "state", s)

	// Increment a counter on the state key.
	key := strings.Join([]string{w.root, "states", string(s)}, ":")
	seq, err := w.client.WithContext(ctx).Incr(key).Result()
	if err != nil {
		return -1, err
	}

	log.Debugw("instances in state", "state", s, "count", seq)

	// If we're within the first 5 instances to write to this state key, we're a
	// supervisor and responsible for keeping it alive. See comment on the
	// analogous logic in Write() for more context.
	if seq <= 5 {
		w.lk.Lock()
		w.keepAliveSet[key] = struct{}{}
		w.lk.Unlock()
	}
	return seq, err
}

// Close closes this Writer, and drops all owned keys immediately, erroring if
// those deletions fail.
func (w *Writer) Close() error {
	w.cancel()

	w.lk.Lock()
	defer w.lk.Unlock()

	w.keepAliveSet = nil

	return nil
}
