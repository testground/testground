package sync

import (
	"context"
	"reflect"

	"github.com/go-redis/redis/v7"
)

const RedisStreamPayloadKey = "payload"

// subscription represents long-lived subscription of a consumer to a subtree.
type subscription struct {
	client  *redis.Client
	w       *Watcher
	subtree *Subtree
	key     string

	outCh reflect.Value
}

// process subscribes to a stream from position 0 performing an indefinite
// blocking XREAD. The XREAD will be cancelled when the subscription is
// cancelled.
func (s *subscription) process() {
	defer s.outCh.Close()

	var (
		key    = s.key
		sendFn = reflect.Value(s.outCh).Send // shorthand
		typ    = s.subtree.PayloadType.Elem()
	)

	startSeq, err := s.client.XLen(key).Result()
	if err != nil {
		s.w.re.SLogger().Errorf("failed to fetch current length of stream: %w", err)
		return
	}

	log := s.w.re.SLogger().With("subtree", s.subtree, "start_seq", startSeq)

	// Get a connection and store its connection ID, so we can unblock it when canceling.
	conn := s.client.Conn()
	defer conn.Close()

	connID, err := conn.ClientID().Result()
	if err != nil {
		s.w.re.SLogger().Errorf("failed to fetch get client ID: %w", err)
		return
	}
	done := make(chan struct{})
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		select {
		case <-s.w.close:
		case <-s.client.Context().Done():
		case <-done:
			// no need to unblock anything.
			return
		}
		// we need a _non_ canceled client for this to work.
		client := s.client.WithContext(context.Background())
		err := client.ClientUnblockWithError(connID).Err()
		if err != nil {
			log.Errorw("failed to kill connection", "error", err)
		}
	}()

	defer func() {
		close(done)
		<-closed
	}()

	args := &redis.XReadArgs{
		Streams: []string{key, "0"},
		Block:   0,
	}

	var last redis.XMessage
	for {
		streams, err := conn.XRead(args).Result()
		if err != nil && err != redis.Nil {
			select {
			case <-s.w.close:
			case <-s.client.Context().Done():
			default:
				// only log an error if we didn't explicitly abort early.
				log.Errorf("failed to XREAD from subtree stream: %w", err)
			}
			return
		}

		if len(streams) > 0 {
			stream := streams[0]
			for _, last = range stream.Messages {
				payload, ok := last.Values[RedisStreamPayloadKey]
				if !ok {
					log.Warnw("received stream entry without payload entry", "payload", last)
					continue
				}

				p, err := decodePayload(payload, typ)
				if err != nil {
					log.Warnf("unable to decode item: %s", err)
					continue
				}
				log.Debugw("delivering item to subscriber", "key", key)
				sendFn(p)
			}
		}

		args.Streams[1] = last.ID
	}
}
