package sync

import (
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
