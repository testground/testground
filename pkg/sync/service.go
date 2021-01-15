package sync

import (
	"context"
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
}

func NewService() Service {
	// TODO
	return &DefaultService{}
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
