package sync

import (
	"fmt"
	"strconv"
	"time"
)

func (s *DefaultService) barrierWorker() {
	defer s.wg.Done()

	var (
		pending []*barrier
		keys    []string

		log = s.log.With("process", "barriers")
	)

	// remove removes the index from both the active and key slices.
	remove := func(b *barrier) {
		key := b.key
		log.Debugw("stopping to monitor barrier", "key", key)

		// drop this barrier from the active set.
		for i, p := range pending {
			if p == b {
				copy(pending[i:], pending[i+1:])
				pending[len(pending)-1] = nil
				pending = pending[:len(pending)-1]
				break
			}
		}

		// drop the key as well.
		for i, k := range keys {
			if k == key {
				copy(keys[i:], keys[i+1:])
				keys[len(keys)-1] = ""
				keys = keys[:len(keys)-1]
				break
			}
		}
	}

	tick := time.NewTicker(1 * time.Second)
	defer func() { tick.Stop() }() // this is a closure because tick gets replaced.

	for {
		cnt := len(pending)
		if cnt == 0 {
			tick.Stop()
			select {
			case <-tick.C:
			default:
			}
		}

		select {
		case newBarrier := <-s.barrierCh:
			b := newBarrier
			pending = append(pending, b)
			keys = append(keys, b.key)

			log.Debugw("added barrier", "new", b.key, "all", keys)
			newBarrier.resultCh <- nil

			if cnt == 0 {
				// we need to reactivate the ticker.
				tick = time.NewTicker(1 * time.Second)
			}

		case <-tick.C:
			log.Debugw("checking barriers", "keys", keys)

		case <-s.ctx.Done():
			log.Debugw("yielding", "pending_barriers", len(pending))
			for _, b := range pending {
				log.Debugw("cancelling pending barrier", "key", b.key)
				b.doneCh <- s.ctx.Err()
				close(b.doneCh)
			}
			pending = nil
			return
		}

		// Test all contexts and forget the barriers whose contexts have fired.
		var del []*barrier
		for _, b := range pending {
			if err := b.ctx.Err(); err != nil {
				log.Debugw("barrier context expired; removing", "key", b.key)
				b.doneCh <- err
				close(b.doneCh)
				del = append(del, b)
			}
		}

		// Prune deleted barriers.
		for _, b := range del {
			remove(b)
		}

		if len(pending) == 0 {
			// nothing to do; loop over; the ticker will be disarmed.
			continue
		}

		// Get the values of all pending states at once, under the context of the DefaultClient.
		log.Debugw("getting barrier values", "keys", keys)
		vals, err := s.rclient.MGet(keys...).Result()
		if err != nil {
			log.Warnw("failed while getting barriers; iteration skipped", "error", err)
			continue
		}

		del = del[:0]
		for i, v := range vals {
			if v == nil {
				continue // nobody else has INCR the barrier yet; skip.
			}

			b := pending[i]
			curr, err := strconv.ParseInt(v.(string), 10, 64)
			if err != nil {
				log.Warnw("failed to parse barrier value", "error", err, "value", v, "key", b.key)
				continue
			}

			// Has the barrier been hit?
			if curr >= b.target {
				log.Debugw("barrier was hit; informing waiters", "key", b.key, "target", b.target, "curr", curr)

				// barrier has been hit; send a nil error on the channel, and close it.
				b.doneCh <- nil
				close(b.doneCh)

				// queue this deletion; otherwise indices won't line up.
				del = append(del, b)
			} else {
				log.Debugw("barrier still unsatisfied", "key", b.key, "target", b.target, "curr", curr)
			}
		}

		for _, b := range del {
			remove(b)
		}
	}
}


func (s *DefaultService) subscriptionWorker() {
	defer s.wg.Done()

	var (
		active  = make(map[string]*subscription)
		rmSubCh = make(chan []*subscription, 1)
	)

	log := s.log.With("process", "subscriptions")

	monitorCtx := func(s *subscription) {
		select {
		case <-s.ctx.Done():
			log.Debugw("context closure detected; removing subscription", "topic", s.topic)
			rmSubCh <- []*subscription{s}
		case <-s.ctx.Done():
			log.Debugw("yielding context monitor routine due to global context closure", "topic", s.topic)
		}
	}

	consumer := &subscriptionConsumer{s: s, log: log, rmSubCh: rmSubCh, notifyCh: make(chan struct{}, 1)}

	var finalErr error // error to broadcast to remaining subscribers upon returning.
	defer func() {
		for _, sub := range active {
			sub.doneCh <- finalErr
			close(sub.doneCh)
			close(sub.outCh)
		}
	}()

	for {
		// Manage subscriptions.
		select {
		case sub := <-s.subCh:
			if _, ok := active[sub.topic]; ok {
				sub.resultCh <- fmt.Errorf("failed to add duplicate subscription")
				continue
			}

			log.Debugw("adding subscription", "topic", sub.topic)

			// interrupt consumer and wait until it yields, before mutating the active set.
			err := consumer.interrupt()
			if err != nil {
				panic(fmt.Sprintf("failed to interrupt consumer when adding subscription; exiting; err: %s", err))
			}

			active[sub.topic] = sub
			go monitorCtx(sub)
			sub.resultCh <- nil

		case subs := <-rmSubCh:
			// interrupt consumer and wait until it yields, before accessing subscriptions.
			err := consumer.interrupt()
			if err != nil {
				panic(fmt.Sprintf("failed to interrupt consumer when removing subscriptions; exiting; err: %s", err))
			}

			for _, s := range subs {
				// this was a planned removal, sending err = nil.
				s.doneCh <- nil
				close(s.doneCh)
				close(s.outCh)

				delete(active, s.topic)
			}

		case <-s.ctx.Done():
			err := consumer.interrupt()
			if err != nil {
				log.Debugw("failed to interrupt consumer when exiting", "error", err)
				finalErr = err
				return
			}
			return
		}

		if len(rmSubCh)+len(s.subCh) > 0 {
			log.Debugf("more subscription control events to consume; looping over")
			// we still have pending items, continue draining before we resume
			// consuming.
			continue
		}

		if len(active) == 0 {
			continue
		}

		log.Debugf("resume consuming")

		// no copy of the active set is needed, as we always interrupt the
		// consumer before mutating the active set.
		consumer.resume(active)
	}
}
