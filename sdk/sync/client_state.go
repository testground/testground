package sync

import (
	"context"
	"fmt"
)

// Barrier sets a barrier on the supplied State that fires when it reaches its
// target value (or higher).
//
// The caller should monitor the channel C returned inside the Barrier object.
// If the barrier is satisfied, the value sent will be nil.
//
// When the context fires, the context's error will be propagated instead. The
// same will occur if the Client's context fires.
//
// If an internal error occurs,
//
// The returned Barrier object contains a channel (C) that fires when the
// barrier reaches its target, is cancelled, or fails.
//
// The Barrier channel is owned by the Client, and by no means should the caller
// close it.
// It is safe to use a non-cancellable context here, like the background
// context. No cancellation is needed unless you want to stop the process early.
func (c *Client) Barrier(ctx context.Context, state State, target int) (*Barrier, error) {
	rp := c.extractor(ctx)
	if rp == nil {
		return nil, ErrNoRunParameters
	}

	b := &Barrier{
		C:      make(chan error, 1),
		state:  state,
		key:    state.Key(rp),
		target: int64(target),
		ctx:    ctx,
	}

	resultCh := make(chan error)
	c.barrierCh <- &newBarrier{b, resultCh}
	err := <-resultCh
	return b, err
}

// MustBarrier calls Barrier, panicking if it errors.
//
// Suitable for shorthanding in test plans.
func (c *Client) MustBarrier(ctx context.Context, state State, required int) *Barrier {
	b, err := c.Barrier(ctx, state, required)
	if err != nil {
		panic(err)
	}
	return b
}

// SignalEntry increments the state counter by one, returning the value of the
// new value of the counter, or an error if the operation fails.
func (c *Client) SignalEntry(ctx context.Context, state State) (after int64, err error) {
	rp := c.extractor(ctx)
	if rp == nil {
		return -1, ErrNoRunParameters
	}

	// Increment a counter on the state key.
	key := state.Key(rp)

	c.log.Debugw("signalling entry to state", "key", key)

	seq, err := c.rclient.Incr(key).Result()
	if err != nil {
		return -1, err
	}

	c.log.Debugw("new value of state", "key", key, "value", seq)
	return seq, err
}

// MustSignalEntry calls SignalEntry, panicking if it errors.
//
// Suitable for shorthanding in test plans.
func (c *Client) MustSignalEntry(ctx context.Context, state State) (current int64) {
	current, err := c.SignalEntry(ctx, state)
	if err != nil {
		panic(err)
	}
	return current
}

// SignalAndWait composes SignalEntry and Barrier, signalling entry on the
// supplied state, and then awaiting until the required value has been reached.
//
// The returned error will be nil if the barrier was met successfully,
// or non-nil if the context expired, or some other error ocurred.
func (c *Client) SignalAndWait(ctx context.Context, state State, target int) (seq int64, err error) {
	rp := c.extractor(ctx)
	if rp == nil {
		return -1, ErrNoRunParameters
	}

	seq, err = c.SignalEntry(ctx, state)
	if err != nil {
		return -1, fmt.Errorf("failed while signalling entry to state %s: %w", state, err)
	}

	b, err := c.Barrier(ctx, state, target)
	if err != nil {
		return -1, fmt.Errorf("failed while setting barrier for state %s, with target %d: %w", state, target, err)
	}
	return seq, <-b.C
}

// MustSignalAndWait calls SignalAndWait, panicking if it errors.
//
// Suitable for shorthanding in test plans.
func (c *Client) MustSignalAndWait(ctx context.Context, state State, target int) (seq int64) {
	seq, err := c.SignalAndWait(ctx, state, target)
	if err != nil {
		panic(err)
	}
	return seq
}
