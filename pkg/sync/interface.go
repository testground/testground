package sync

import (
	"context"
)

type Service interface {
	Publish(ctx context.Context, topic string, payload interface{}) (seq int64, err error)
	Subscribe(ctx context.Context, topic string) (*Subscription, error)
	Barrier(ctx context.Context, state string, target int64) error
	SignalEntry(ctx context.Context, state string) (after int64, err error)
	SignalEvent(ctx context.Context, key string, event interface{}) error
}

type Subscription struct {
	outCh  chan interface{}
	doneCh chan error
}
