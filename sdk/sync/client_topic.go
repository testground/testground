package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/go-redis/redis/v7"
)

// Publish publishes an item on the supplied topic. The payload type must match
// the payload type on the Topic; otherwise Publish will error.
//
// This method returns synchronously, once the item has been published
// successfully, returning the sequence number of the new item in the ordered
// topic, or an error if one ocurred, starting with 1 (for the first item).
//
// If error is non-nil, the sequence number must be disregarded.
func (c *Client) Publish(ctx context.Context, topic *Topic, payload interface{}) (seq int64, err error) {
	rp := c.extractor(ctx)
	if rp == nil {
		return -1, ErrNoRunParameters
	}

	log := c.log.With("topic", topic.Name)
	log.Debugw("publishing item on topic", "payload", payload)

	if !topic.validatePayload(payload) {
		err = fmt.Errorf("invalid payload type; expected: [*]%s, was: %T", topic.Type, payload)
		return -1, err
	}

	// Serialize the payload.
	bytes, err := json.Marshal(payload)
	if err != nil {
		err = fmt.Errorf("failed while serializing payload: %w", err)
		return -1, err
	}

	log.Debugw("serialized json payload", "json", string(bytes))

	key := topic.Key(rp)

	log.Debugw("resolved key for publish", "key", key)

	// Perform a Redis transaction, adding the item to the stream and fetching
	// the XLEN of the stream.
	args := new(redis.XAddArgs)
	args.ID = "*"
	args.Stream = key
	args.Values = map[string]interface{}{RedisPayloadKey: bytes}

	pipe := c.rclient.TxPipeline()
	_ = pipe.XAdd(args)
	xlen := pipe.XLen(key)

	_, err = pipe.ExecContext(ctx)
	if err != nil {
		log.Debugw("failed to publish item", "error", err)
		return -1, err
	}

	seq = xlen.Val()
	c.log.Debugw("successfully published item; sequence number obtained", "seq", seq)

	return seq, err
}

// MustPublish calls Publish, panicking if it errors.
//
// Suitable for shorthanding in test plans.
func (c *Client) MustPublish(ctx context.Context, topic *Topic, payload interface{}) (seq int64) {
	seq, err := c.Publish(ctx, topic, payload)
	if err != nil {
		panic(err)
	}
	return seq
}

// Subscribe subscribes to a topic, consuming ordered, typed elements from
// index 0, and sending them to channel ch.
//
// The supplied channel must be buffered, and its type must be a value or
// pointer type matching the topic type. If these conditions are unmet, this
// method will error immediately.
//
// The caller must consume from this channel promptly; failure to do so will
// backpressure the Client's subscription event loop.
func (c *Client) Subscribe(ctx context.Context, topic *Topic, ch interface{}) (*Subscription, error) {
	rp := c.extractor(ctx)
	if rp == nil {
		return nil, ErrNoRunParameters
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := c.ctx.Err(); err != nil {
		return nil, err
	}

	chv := reflect.ValueOf(ch)
	if k := chv.Kind(); k != reflect.Chan {
		return nil, fmt.Errorf("value is not a channel: %T", ch)
	}

	if chv.Cap() == 0 {
		return nil, fmt.Errorf("channel is not buffered")
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

	ttyp := topic.Type
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
}

// MustSubscribe calls Subscribe, panicking if it errors.
//
// Suitable for shorthanding in test plans.
func (c *Client) MustSubscribe(ctx context.Context, topic *Topic, ch interface{}) (sub *Subscription) {
	sub, err := c.Subscribe(ctx, topic, ch)
	if err != nil {
		panic(err)
	}
	return sub
}
