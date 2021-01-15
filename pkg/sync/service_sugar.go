package sync

import (
	"context"
	"fmt"
)

type sugarOperations struct {
	Service
}

// PublishAndWait composes Publish and a Barrier. It first publishes the
// provided payload to the specified topic, then awaits for a barrier on the
// supplied state to reach the indicated target.
//
// If any operation fails, PublishAndWait short-circuits and returns a non-nil
// error and a negative sequence. If Publish succeeds, but the Barrier fails,
// the seq number will be greater than zero.
func (c *sugarOperations) PublishAndWait(ctx context.Context, topic string, payload interface{}, state string, target int64) (seq int64, err error) {
	seq, err = c.Publish(ctx, topic, payload)
	if err != nil {
		return -1, err
	}

	err = c.Barrier(ctx, state, target)
	return seq, err
}

// SignalAndWait composes SignalEntry and Barrier, signalling entry on the
// supplied state, and then awaiting until the required value has been reached.
//
// The returned error will be nil if the barrier was met successfully,
// or non-nil if the context expired, or some other error occurred.
func (c *sugarOperations) SignalAndWait(ctx context.Context, state string, target int64) (seq int64, err error) {
	seq, err = c.SignalEntry(ctx, state)
	if err != nil {
		return -1, fmt.Errorf("failed while signalling entry to state %s: %w", state, err)
	}

	err = c.Barrier(ctx, state, target)
	return seq, err
}

/*

// PublishSubscribe publishes the payload on the supplied Topic, then subscribes
// to it, sending payloads to the supplied channel.
//
// If any operation fails, PublishSubscribe short-circuits and returns a non-nil
// error and a negative sequence. If Publish succeeds, but Subscribe fails,
// the seq number will be greater than zero, but the returned Subscription will
// be nil, and the error, non-nil.
func (c *sugarOperations) PublishSubscribe(ctx context.Context, topic *Topic, payload interface{}, ch interface{}) (seq int64, sub *Subscription, err error) {
	seq, err = c.Publish(ctx, topic, payload)
	if err != nil {
		return -1, nil, err
	}
	sub, err = c.Subscribe(ctx, topic, ch)
	if err != nil {
		return seq, nil, err
	}
	return seq, sub, err
}

*/
