package sync

import (
	"context"
)

type Service interface {
	Publish(ctx context.Context, topic string, payload interface{}) (seq int64, err error)
	Subscribe(ctx context.Context, topic string, ch chan interface{}) (*Subscription, error)
	PublishAndWait(ctx context.Context, topic string, payload interface{}, state string, target int64) (seq int64, err error)
	// PublishSubscribe(ctx context.Context, topic string, payload interface{}, ch interface{}) (seq int64, sub *Subscription, err error)

	Barrier(ctx context.Context, state string, target int64) error
	SignalEntry(ctx context.Context, state string) (after int64, err error)
	SignalAndWait(ctx context.Context, state string, target int64) (seq int64, err error)
	SignalEvent(ctx context.Context, key string, event interface{}) error
}

// Subscription represents a receive channel for data being published in a
// Topic.
type Subscription struct {
	ctx    context.Context
	outCh  chan interface{}
	doneCh chan error
	resultCh chan error

	// sendFn performs a select over outCh and the context, and returns true if
	// we sent the value, or false if the context fired.
	sendFn func(interface{}) (sent bool)

	topic  string
	lastid string
}

func (s *Subscription) Done() <-chan error {
	return s.doneCh
}
