package sync

import (
	"fmt"
	"github.com/go-redis/redis/v7"
	"go.uber.org/zap"
	"sync/atomic"
	"time"
)

const (
	StateStopped      = 0
	StateInterrupting = 1
	StateRunning      = 2
)

type subscriptionConsumer struct {
	state    int32         // atomic
	notifyCh chan struct{} // must be buffered with cap=1

	conn     *redis.Conn
	clientid int64 // atomic

	s   *DefaultService
	log *zap.SugaredLogger

	connErr error
	rmSubCh chan<- []*Subscription
}

func (sc *subscriptionConsumer) interrupt() error {
	sc.log.Debugw("interrupting consumer")

	if !atomic.CompareAndSwapInt32(&sc.state, StateRunning, StateInterrupting) {
		switch atomic.LoadInt32(&sc.state) {
		case StateInterrupting:
			panic("duplicate call to interrupt")
		case StateStopped:
			sc.log.Debug("attempted to interrupt a stopped consumer")
			return nil
		default:
			panic("invalid state")
		}
	}

	// attempt to unblock the client every 500ms.
	for ok := true; ok; ok = atomic.LoadInt32(&sc.state) != StateStopped {
		clientid := atomic.LoadInt64(&sc.clientid)
		sc.log.Debugw("attempting to unblock connection", "client_id", clientid)
		unblocked, err := sc.s.rclient.ClientUnblock(clientid).Result()
		if err != nil {
			return err
		}
		if unblocked == 0 {
			sc.log.Debugw("no connections interrupted", "client_id", clientid)
		}
		select {
		case <-sc.notifyCh:
			return nil
		case <-time.After(500 * time.Millisecond):
		}
	}
	return nil
}

func (sc *subscriptionConsumer) resume(active map[string]*Subscription) {
	if len(active) == 0 {
		sc.log.Debugw("no subscriptions to consume, yielding")
		return
	}

	// unexpected consumer state; either the caller called resume twice, or
	// called resume while an interruption was taking place.
	if !atomic.CompareAndSwapInt32(&sc.state, StateStopped, StateRunning) {
		st := atomic.LoadInt32(&sc.state)
		panic(fmt.Sprintf("fatal subscription consumer state: %d", st))
	}

	go sc.consume(active)
}

func (sc *subscriptionConsumer) consume(active map[string]*Subscription) {
	defer func() { sc.notifyCh <- struct{}{} }()

	defer atomic.StoreInt32(&sc.state, StateStopped)

	// slurp any unconsumed stopped notifications.
	select {
	case <-sc.notifyCh:
	default:
	}

RegenerateConnection:
	// connection (re-)creation logic; runs if the last connection errored, or
	// if we're being called for the first time and therefore don't have a
	// connection.
	var clientid int64
	for atomic.LoadInt32(&sc.state) == StateRunning && (sc.connErr != nil || sc.conn == nil) {
		sc.log.Debugf("creating subscription connection")

		if sc.conn != nil {
			_ = sc.conn.Close()
		}
		sc.conn = sc.s.rclient.Conn()
		clientid, sc.connErr = sc.conn.ClientID().Result()
		if sc.connErr == nil {
			atomic.StoreInt64(&sc.clientid, clientid)
			sc.log.Debugw("subscription connection created", "client_id", sc.clientid)
			break
		}

		sc.log.Debugw("failed to create subscription connection", "error", sc.connErr)

		select {
		case <-time.After(1 * time.Second):
		case <-sc.s.ctx.Done():
			return // we're done.
		}
	}

	// Abort if an interruption has been requested.
	if atomic.LoadInt32(&sc.state) != StateRunning {
		return
	}

	// We now have a good connection; let's run the XREAD loop.
	cnt, i := len(active), 0
	rev := make(map[string]int, cnt) // reverse idx storing keys => i, to update args efficiently

	args := new(redis.XReadArgs)
	args.Streams = make([]string, cnt*2)
	args.Block = 0  // block forever if no elements are available
	args.Count = 10 // max 10 elements per stream

	for k, s := range active {
		// STREAMS stream1 stream2 stream3 seek_key1 seek_key2 seek_key3
		args.Streams[i] = k
		args.Streams[cnt+i] = s.lastid
		rev[k] = cnt + i
		i++
	}

	var rmSub []*Subscription
	for atomic.LoadInt32(&sc.state) == StateRunning {
		sc.log.Debugw("XREAD streams", "streams", args.Streams)

		streams, err := sc.conn.XRead(args).Result()
		if err != nil {
			sc.connErr = err
			goto RegenerateConnection
		}

		for _, xr := range streams {
			if len(xr.Messages) == 0 {
				sc.log.Debugw("XREAD response: stream with no messages", "key", xr.Stream)
				continue
			}

			sub, ok := active[xr.Stream]
			if !ok {
				sc.log.Debugw("XREAD response: rcvd messages for a stream we're not subscribed to", "key", xr.Stream)
				continue
			}

			for _, msg := range xr.Messages {
				payload := msg.Values[RedisPayloadKey]

				sc.log.Debugw("dispatching message to subscriber", "key", xr.Stream, "id", msg.ID)

				if sent := sub.sendFn(payload); !sent {
					// we could not send value because context fired.
					// skip all further messages on this stream, and queue for
					// removal.
					sc.log.Debugw("context was closed when dispatching message to subscriber; rm subscription", "key", xr.Stream, "id", msg.ID)
					rmSub = append(rmSub, sub)
					break
				}
				sub.lastid = msg.ID
			}
		}

		if len(rmSub) > 0 {
			// we have subscriptions to remove, so let the coordinator loop
			// know, and yield.
			sc.rmSubCh <- rmSub
			break
		}

		// Update the XREAD request.
		for k, v := range active {
			args.Streams[rev[k]] = v.lastid
		}
	}
}
