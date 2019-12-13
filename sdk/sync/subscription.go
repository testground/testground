package sync

import (
	"reflect"
	"sync"
	"time"

	"github.com/go-redis/redis"
)

// subscription represents long-lived subscription of a consumer to a subtree.
type subscription struct {
	lk      sync.RWMutex
	root    string
	w       *Watcher
	subtree *Subtree
	client  *redis.Client
	outCh   reflect.Value
	closeCh chan struct{}
}

func (s *subscription) start() {
	log := s.w.re.SLogger().With("subtree", s.subtree)
	sendFn := reflect.Value(s.outCh).Send // shorthand
	typ := s.subtree.PayloadType.Elem()

	go func() {
		id := "0"

		for !s.isClosed() {
			streams, err := s.client.XRead(&redis.XReadArgs{
				Streams: []string{s.root, id},
				Block:   time.Second,
			}).Result()

			if err != nil {
				if err != redis.Nil {
					log.Error(err)
				}
				continue
			}

			for _, stream := range streams {
				for _, message := range stream.Messages {
					id = message.ID
					p, err := decodePayload(message.Values["payload"], typ)
					if err != nil {
						log.Errorf("unable to decode item: %s", err)
						panic(err)
					}

					if !s.isClosed() {
						sendFn(p)
					}
				}
			}
		}
	}()
}

func (s *subscription) isClosed() bool {
	select {
	case <-s.closeCh:
		return true
	default:
		return false
	}
}

func (s *subscription) stop() {
	v := reflect.Value(s.outCh)
	v.Close()
}
