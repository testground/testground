package sync

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v7"
	"go.uber.org/zap"
	"sync"
)

type DefaultService struct {
	*sugarOperations

	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	rclient *redis.Client
	log     *zap.SugaredLogger

	barrierCh chan *barrier
	subCh     chan *Subscription
}

func NewService(ctx context.Context, log *zap.SugaredLogger, cfg *RedisConfiguration) (Service, error) {
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
		subCh:     make(chan *Subscription),
	}
	
	s.sugarOperations = &sugarOperations{s}

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
