package sync

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/go-redis/redis/v7"
)

const RedisStreamPayloadKey = "payload"

// subscription represents long-lived subscription of a consumer to a subtree.
type subscription struct {
	lk sync.RWMutex

	client  *redis.Client
	w       *Watcher
	subtree *Subtree
	key     string

	outCh reflect.Value

	// connCh stores the connection ID of the subscription's conn. Consuming
	// from here should always return a value, either a connection ID or -1 if
	// no connection was created (e.g. error situation).
	connCh  chan int64
	closeCh chan struct{}
	closeWg sync.WaitGroup
	errCh   chan error
}

func (s *subscription) isClosed() bool {
	select {
	case <-s.closeCh:
		return true
	default:
		return false
	}
}

// process subscribes to a stream from position 0 performing an indefinite
// blocking XREAD. The XREAD will be cancelled when the subscription is
// cancelled.
func (s *subscription) process() {
	defer s.closeWg.Done()
	defer s.outCh.Close()

	var (
		key    = s.key
		sendFn = reflect.Value(s.outCh).Send // shorthand
		typ    = s.subtree.PayloadType.Elem()
	)

	startSeq, err := s.client.XLen(key).Result()
	if err != nil {
		s.connCh <- -1
		s.errCh <- fmt.Errorf("failed to fetch current length of stream: %w", err)
		return
	}

	log := s.w.re.SLogger().With("subtree", s.subtree, "start_seq", startSeq)

	// Get a connection and store its connection ID, so that stop() can unblock
	// it upon closure.
	conn := s.client.Conn()
	defer conn.Close()

	id, err := conn.ClientID().Result()
	if err != nil {
		s.connCh <- -1
		s.errCh <- fmt.Errorf("failed to get the current conn id: %w", err)
		return
	}

	// store the conn ID in the channel.
	s.connCh <- id

	args := &redis.XReadArgs{
		Streams: []string{key, "0"},
		Block:   0,
	}

	var last redis.XMessage
	for {
		streams, err := conn.XRead(args).Result()
		if err != nil && err != redis.Nil {
			s.errCh <- fmt.Errorf("failed to XREAD from subtree stream: %w", err)
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

		// Return if the subscription has been closed.
		if s.isClosed() {
			s.errCh <- nil
			return
		}

		args.Streams[1] = last.ID
	}

	s.errCh <- nil
}

// stop stops this subcription.
func (s *subscription) stop() error {
	if s.isClosed() {
		return fmt.Errorf("called stop on subscription multiple times")
	}

	close(s.closeCh)

	if connID := <-s.connCh; connID != -1 {
		// this subscription has a connection associated with it.
		if err := s.client.ClientUnblock(connID).Err(); err != nil {
			s.lk.Unlock()
			return fmt.Errorf("failed to unblock connection: %w", err)
		}
	}

	s.closeWg.Wait()
	return <-s.errCh
}
