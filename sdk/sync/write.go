package sync

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
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

// Write writes a payload in the sync tree for the test.
//
// It panics if the payload's type does not match the expected type for the
// subtree.
//
// If the actual write on the sync service fails, this method returns an error.
//
// Else, if all succeeds, it returns the ordinal sequence number of this entry
// within the subtree.
func (w *Writer) Write(subtree *Subtree, payload interface{}) (seq int64, err error) {
	if err = subtree.AssertType(reflect.ValueOf(payload).Type()); err != nil {
		return -1, err
	}

	// Serialize the payload.
	bytes, err := json.Marshal(payload)
	if err != nil {
		return -1, err
	}

	// Calculate the index key. This key itself holds a set pointing to all
	// children. It also nests the seq ordinal and the actual children payloads.
	idx := w.root + ":" + subtree.GroupKey

	// Claim an seq by incrementing the :seq subkey.
	seq, err = w.client.Incr(idx + ":seq").Result()
	if err != nil {
		return -1, err
	}

	// Payload key segments:
	// run:<runid>:plan:<plan_name>:case:<case_name>:<group_key>:<payload_key>:<seq>
	// e.g.
	// run:123:plan:dht:case:lookup_peers:nodes:QmPeer:417
	payloadKey := strings.Join([]string{idx, subtree.KeyFunc(payload), strconv.Itoa(int(seq))}, ":")

	// Perform a transaction setting the payload key and adding it to the index
	// set.
	err = w.client.Watch(func(tx *redis.Tx) error {
		_, err := tx.Pipelined(func(pipe redis.Pipeliner) error {
			pipe.Set(payloadKey, bytes, TTL)
			pipe.SAdd(idx, payloadKey)
			return nil
		})
		return err
	})

	if err != nil {
		return -1, err
	}

	// Update the ownset.
	w.lk.Lock()
	os := w.ownsets[subtree.GroupKey]
	w.ownsets[subtree.GroupKey] = append(os, payloadKey)
	w.lk.Unlock()

	return seq, err
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
