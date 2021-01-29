package sync

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v7"
	"go.uber.org/zap"
	"sync"
)

type DefaultService struct {
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	rclient *redis.Client
	log     *zap.SugaredLogger

	barrierCh chan *barrier
	subCh     chan *subscription
}

func NewService(ctx context.Context, log *zap.SugaredLogger, cfg *RedisConfiguration) (*DefaultService, error) {
	rclient, err := redisClient(ctx, log, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create redis client: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	s := &DefaultService{
		ctx:       ctx,
		cancel:    cancel,
		log:       log,
		rclient:   rclient,
		barrierCh: make(chan *barrier),
		subCh:     make(chan *subscription),
	}

	s.wg.Add(2)
	go s.barrierWorker()
	go s.subscriptionWorker()

	return s, nil
}

// Close closes this service, cancels ongoing operations, and releases resources.
func (s *DefaultService) Close() error {
	s.cancel()
	s.wg.Wait()

	return s.rclient.Close()
}

// barrier represents a barrier over a State. A Barrier is a synchronisation
// checkpoint that will fire once the `target` number of entries on that state
// have been registered.
type barrier struct {
	ctx      context.Context
	key      string
	target   int64
	doneCh   chan error
	resultCh chan error
}

// subscription represents a receive channel for data being published in a
// Topic.
type subscription struct {
	ctx      context.Context
	outCh    chan interface{}
	doneCh   chan error
	resultCh chan error

	// sendFn performs a select over outCh and the context, and returns true if
	// we sent the value, or false if the context fired.
	sendFn func(interface{}) (sent bool)

	topic  string
	lastid string
}
