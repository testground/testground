package sync

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/go-redis/redis"
)

// subscription represents long-lived subscription of a consumer to a subtree.
type subscription struct {
	lk sync.RWMutex

	root      string
	startMVCC int

	w       *Watcher
	subtree *Subtree
	client  *redis.Client
	ps      *redis.PubSub

	outCh typedChan
	inCh  <-chan *redis.Message
}

// start brings the state of the subscription forward to match the state of the
// store. It does so in two phases:
//
//   1. Replay: read the current state of the subtree from Redis, and
//      replay it to the consumer.
//   2. Batch fast-forward: reconcile the events we received during the replay
//      phase. Drain the queue and switch to online consumption.
func (s *subscription) start() error {
	s.ps = s.client.PSubscribe("__keyspace@0__:" + s.root + ":*")
	s.inCh = s.ps.Channel()

	// Get the current counter; we'll only process entries prior or equal to
	// this counter. This is a way of "freezing" state.
	mvcc, err := s.client.Get(s.root + ":mvcc").Int()
	if err != nil && err != redis.Nil {
		return err
	}

	s.startMVCC = mvcc

	go s.process()
	return nil
}

// process does the heavy-lifting of this subscription.
func (s *subscription) process() {
	log := s.w.re.SLogger().With("subtree", s.subtree, "start_mvcc", s.startMVCC)

	// Sync: state can slide through two pathways while we synchronize. To
	// mitigate that, we use a counter that acts like an MVCC sentinel for
	// syncing. Assuming that entries are immutable, we read entries from the
	// iterator and discard those that have an MVCC higher than the one we
	// initialized with. Newer entries will arrive as notifications.
	var (
		keys   []string                    // gets reassigned on every iteration.
		dead   = make(map[string]struct{}) // accumulates dead/expired elements for pruning.
		cursor uint64                      // current state of the cursor
		err    error
		sendFn = reflect.Value(s.outCh).Send // shorthand
		typ    = s.subtree.PayloadType.Elem()
	)

	log.Infow("syncing subscription")
	log.Infow("replaying state")

	//---------------------------
	// --- FAST-FORWARD PHASE ---
	//---------------------------

	for i := 0; ; i++ {
		// Read elements from the set in increments of 1000.
		keys, cursor, err = s.client.SScan(s.root, cursor, "", 1000).Result()
		if err != nil {
			panic(err)
		}

		log.Debugw("replay iteration", "iteration", i, "key_count", len(keys))

		// Filter keys to retain only those whose MVCC is lower or equal to the
		// start MVCC.
		filtered := make([]string, 0, len(keys))
		for _, k := range keys {
			if mvcc := mvccFromKey(k); mvcc > s.startMVCC {
				continue
			}
			filtered = append(filtered, k)
		}
		keys = filtered
		filtered = nil

		log.Debugf("eligible key count: %d", len(keys))

		// Fetch all the indexed keys; we need to verify they exist. Some may
		// have expired.
		if len(keys) > 0 {
			res, err := s.client.MGet(keys...).Result()
			if err != nil {
				panic(err)
			}

			// Iterate over keys.
			for i, k := range keys {
				if res[i] == nil {
					// Mark this key as dead. Do not send in consumer channel.
					dead[k] = struct{}{}
					log.Debugw("found dead key in group set", "key", k)
					continue
				}

				// Deserialize the value, and publish to the consumer.
				log.Debugw("publishing item to subscriber", "key", k)
				payload, err := decodePayload(res[i], typ)
				if err != nil {
					panic(err)
				}
				sendFn(payload)
			}
		}

		if cursor == 0 {
			// We've exhausted the scan. Break out of the loop.
			break
		}
	}

	// Free this slice.
	keys = nil

	// Housekeeping. Prune elements that are still in the index but no
	// longer exist (i.e. expired elements).
	// CAUTION: these events will arrive as deletions on the set.
	if l := len(dead); l > 0 {
		go func() {
			keys := make([]string, 0, l)
			for k := range dead {
				keys = append(keys, k)
			}
			del, err := s.client.SRem(s.root, keys).Result()
			if err != nil {
				panic(fmt.Sprintf("pruning dead keys from index key failed: %s", err))
			}
			log.Infow("successfully pruned dead keys", "prunable", len(dead), "pruned", del)
			dead = nil // let the slice go.
		}()
	}

	// Function to extract the key from a pubsub notification.
	extractKey := func(msg *redis.Message) string {
		key := strings.TrimPrefix(msg.Channel, "__keyspace@0__:")
		return key
	}

	// --- BATCH FAST-FORWARD ---

	log.Infow("batch fast-forwarding")

	var pendingKeys []string
Loop:
	for {
		select {
		case msg := <-s.inCh:
			if key := extractKey(msg); key != "" && msg.Payload == "set" {
				pendingKeys = append(pendingKeys, key)
			}
		default:
			break Loop
		}
	}

	log.Infof("received %d notifications during replay", len(pendingKeys))

	// Do a multi-get for the pending keys.
	if len(pendingKeys) > 0 {
		pendingVals, err := s.client.MGet(pendingKeys...).Result()
		if err != nil {
			panic(err)
		}
		for i, v := range pendingVals {
			if v == nil {
				continue
			}

			log.Debugw("publishing item to subscriber", "key", pendingKeys[i])

			p, err := decodePayload(v, typ)
			if err != nil {
				log.Warnf("unable to decode item: %s", err)
				panic(err)
			}

			sendFn(p)
		}
		pendingKeys, pendingVals = nil, nil
	}

	log.Infow("now consuming actively")

	// --- FORWARD CONSUMPTION ---

	for msg := range s.inCh {
		log.Debugw("received keyspace notification", "message", msg)
		switch msg.Payload {
		case "set":
			key := extractKey(msg)
			if key == "" {
				continue
			}

			// TODO: batch gets rather than individually getting each key.
			v, err := s.client.Get(key).Result()
			switch err {
			case nil:
				p, err := decodePayload(v, typ)
				if err != nil {
					log.Warnf("unable to decode item: %s", err)
					panic(err)
				}
				sendFn(p)

			case redis.Nil:
				// TODO: log we received a notification for a key that
				// disappeared.

			default:
				panic(err)
			}

		default:
		}
	}
}

// stop stops this subcription.
func (s *subscription) stop() error {
	if err := s.ps.Unsubscribe("__keyspace@0__:" + s.subtree.GroupKey); err != nil {
		return err
	}

	if err := s.ps.Close(); err != nil {
		return err
	}

	v := reflect.Value(s.outCh)
	v.Close()
	return nil
}
