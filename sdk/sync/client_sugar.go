package sync

import "context"

// PublishAndWait composes Publish and a Barrier. It first publishes the
// provided payload to the specified topic, then awaits for a barrier on the
// supplied state to reach the indicated target.
//
// If any operation fails, PublishAndWait short-circuits and returns a non-nil
// error and a negative sequence. If Publish succeeds, but the Barrier fails,
// the seq number will be greater than zero.
func (c *Client) PublishAndWait(ctx context.Context, topic *Topic, payload interface{}, state State, target int) (seq int64, err error) {
	seq, err = c.Publish(ctx, topic, payload)
	if err != nil {
		return -1, err
	}

	b, err := c.Barrier(ctx, state, target)
	if err != nil {
		return seq, err
	}

	<-b.C
	return seq, err
}

// MustPublishAndWait calls PublishAndWait, panicking if it errors.
//
// Suitable for shorthanding in test plans.
func (c *Client) MustPublishAndWait(ctx context.Context, topic *Topic, payload interface{}, state State, target int) (seq int64) {
	seq, err := c.PublishAndWait(ctx, topic, payload, state, target)
	if err != nil {
		panic(err)
	}
	return seq
}

// PublishSubscribe publishes the payload on the supplied Topic, then subscribes
// to it, sending paylods to the supplied channel.
//
// If any operation fails, PublishSubscribe short-circuits and returns a non-nil
// error and a negative sequence. If Publish succeeds, but Subscribe fails,
// the seq number will be greater than zero, but the returned Subscription will
// be nil, and the error, non-nil.
func (c *Client) PublishSubscribe(ctx context.Context, topic *Topic, payload interface{}, ch interface{}) (seq int64, sub *Subscription, err error) {
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

// MustPublishSubscribe calls PublishSubscribe, panicking if it errors.
//
// Suitable for shorthanding in test plans.
func (c *Client) MustPublishSubscribe(ctx context.Context, topic *Topic, payload interface{}, ch interface{}) (seq int64, sub *Subscription) {
	seq, sub, err := c.PublishSubscribe(ctx, topic, payload, ch)
	if err != nil {
		panic(err)
	}
	return seq, sub
}
