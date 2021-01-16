package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v7"
	"reflect"
)

// Publish publishes an item on the supplied topic. The payload type must match
// the payload type on the Topic; otherwise Publish will error.
//
// This method returns synchronously, once the item has been published
// successfully, returning the sequence number of the new item in the ordered
// topic, or an error if one occurred, starting with 1 (for the first item).
//
// If error is non-nil, the sequence number must be disregarded.
func (s *DefaultService) Publish(ctx context.Context, topic string, payload interface{}) (seq int64, err error) {
	log := s.log.With("topic", topic)
	log.Debugw("publishing item on topic", "payload", payload)

	// Serialize the payload.
	bytes, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("failed while serializing payload: %w", err)
	}

	log.Debugw("serialized json payload", "json", string(bytes))

	// Perform a Redis transaction, adding the item to the stream and fetching
	// the XLEN of the stream.
	args := new(redis.XAddArgs)
	args.ID = "*"
	args.Stream = topic
	args.Values = map[string]interface{}{RedisPayloadKey: bytes}

	pipe := s.rclient.TxPipeline()
	_ = pipe.XAdd(args)
	xlen := pipe.XLen(topic)

	_, err = pipe.ExecContext(ctx)
	if err != nil {
		log.Debugw("failed to publish item", "error", err)
		return 0, err
	}

	seq = xlen.Val()
	s.log.Debugw("successfully published item; sequence number obtained", "seq", seq)
	return seq, nil
}

func (s *DefaultService) Subscribe(ctx context.Context, topic string) (sub *Subscription, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := s.ctx.Err(); err != nil {
		return nil, err
	}

	chv := reflect.ValueOf(ch)
	if k := chv.Kind(); k != reflect.Chan {
		return nil, fmt.Errorf("value is not a channel: %T", ch)
	}

	// compare the naked types; this makes the subscription work with pointer
	// or value channels.
	var deref bool
	chtyp := chv.Type().Elem()
	if chtyp.Kind() == reflect.Ptr {
		chtyp = chtyp.Elem()
	} else {
		deref = true
	}

	ttyp := topic.typ
	if ttyp.Kind() == reflect.Ptr {
		ttyp = ttyp.Elem()
	}
	if chtyp != ttyp {
		return nil, fmt.Errorf("unexpected channel type; expected: [*]%s, was: %s", ttyp, chtyp)
	}

	key := topic.Key(rp)

	// sendFn is a closure that sends an element into the supplied ch,
	// performing necessary pointer to value conversions if necessary.
	//
	// sendFn will block if the receiver is not consuming from the channel.
	// If the context is closed, the send will be aborted, and the closure will
	// return a false value.
	sendFn := func(v reflect.Value) (sent bool) {
		if deref {
			v = v.Elem()
		}
		cases := []reflect.SelectCase{
			{Dir: reflect.SelectSend, Chan: chv, Send: v},
			{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ctx.Done())},
		}
		_, _, ctxFired := reflect.Select(cases)
		return !ctxFired
	}

	sub := &Subscription{
		ctx:    ctx,
		outCh:  chv,
		doneCh: make(chan error, 1),
		sendFn: sendFn,
		topic:  topic,
		key:    key,
		lastid: "0",
	}

	resultCh := make(chan error)
	c.newSubCh <- &newSubscription{sub, resultCh}
	err := <-resultCh
	return sub, err

	return nil
}
