package sync

import (
	"encoding/json"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/ipfs/testground/sdk/runtime"

	"github.com/go-redis/redis"
)

var (
	// TTL is the expiry of the records this writer inserts.
	TTL = 10 * time.Second

	// RefreshPeriod is half the TTL. The Writer refreshes records it owns with
	// this frequency.
	RefreshPeriod = TTL / 2
)

// Writer offers an API to write objects to the sync tree for a running test.
type Writer struct {
	lk     sync.RWMutex
	client *redis.Client
	root   string
	// ownsets tracks the keys we own, grouped by GroupKey. We use this to
	// refresh the TTL in the background while we're alive.
	ownsets map[string][]string
	doneCh  chan struct{}
}

// NewWriter creates a new Writer for a particular test run.
func NewWriter(runenv *runtime.RunEnv) (w *Writer, err error) {
	client, err := redisClient(runenv)
	if err != nil {
		return nil, err
	}

	w = &Writer{
		client:  client,
		root:    basePrefix(runenv),
		doneCh:  make(chan struct{}),
		ownsets: make(map[string][]string),
	}

	go w.refreshOwned()
	return w, nil
}

// refreshOwned runs a loop that refreshes owned keys every `RefreshPeriod`.
// It should be launched as a goroutine.
func (w *Writer) refreshOwned() {
Loop:
	select {
	case <-time.After(RefreshPeriod):
		w.lk.RLock()
		// TODO: do this in a transaction. We risk the loop overlapping with the
		// refresh period, and all kinds of races. We need to be adaptive here.
		for _, os := range w.ownsets {
			for _, k := range os {
				if err := w.client.Expire(k, TTL).Err(); err != nil {
					w.lk.RUnlock()
					panic(err)
				}
			}
		}
		w.lk.RUnlock()
		goto Loop

	case <-w.doneCh:
		return
	}
}

// Write writes a payload in the sync tree for the test. It panics if the
// payload's type does not match the expected type for the subtree. If the
// actual write on the sync service fails, this method returns an error.
func (w *Writer) Write(subtree *Subtree, payload interface{}) (err error) {
	if err = subtree.AssertType(reflect.ValueOf(payload).Type()); err != nil {
		return err
	}

	// Serialize the payload.
	bytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Calculate the index key.
	idx := w.root + ":" + subtree.GroupKey

	// Claim an seq.
	seq, err := w.client.Incr(idx + ":seq").Result()
	if err != nil {
		return err
	}

	key := w.root + ":" + subtree.PathFunc(payload) + ":" + strconv.Itoa(int(seq))

	// Perform a transaction setting the key and adding it to the index.
	err = w.client.Watch(func(tx *redis.Tx) error {
		_, err := tx.Pipelined(func(pipe redis.Pipeliner) error {
			pipe.Set(key, bytes, TTL)
			pipe.SAdd(idx, key)
			return nil
		})
		return err
	})

	if err != nil {
		return err
	}

	// Update the ownset.
	w.lk.Lock()
	defer w.lk.Unlock()

	os := w.ownsets[subtree.GroupKey]
	w.ownsets[subtree.GroupKey] = append(os, key)

	return err
}

// Close closes this writer, and drops all owned keys immediately.
func (w *Writer) Close() error {
	close(w.doneCh)

	w.lk.Lock()
	defer w.lk.Unlock()

	// Drop all keys owned by this writer.
	for g, os := range w.ownsets {
		if err := w.client.SRem(g, os).Err(); err != nil {
			return err
		}
		if err := w.client.Del(os...).Err(); err != nil {
			return err
		}
	}

	return nil
}
