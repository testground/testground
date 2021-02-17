package sync

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v7"
	"go.uber.org/zap"
	"sync"
	"time"
)

const (
	RedisPayloadKey = "p"
)

var DefaultRedisOpts = redis.Options{
	MinIdleConns:       2,               // allow the pool to downsize to 0 conns.
	PoolSize:           5,               // one for subscriptions, one for nonblocking operations.
	PoolTimeout:        3 * time.Minute, // amount of time a waiter will wait for a conn to become available.
	MaxRetries:         30,
	MinRetryBackoff:    1 * time.Second,
	MaxRetryBackoff:    3 * time.Second,
	DialTimeout:        10 * time.Second,
	ReadTimeout:        10 * time.Second,
	WriteTimeout:       10 * time.Second,
	IdleCheckFrequency: 30 * time.Second,
	MaxConnAge:         2 * time.Minute,
}

type RedisConfiguration struct {
	Port int
	Host string
}

type RedisService struct {
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	rclient *redis.Client
	log     *zap.SugaredLogger

	barrierCh chan *barrier
	subCh     chan *subscription
}

func NewRedisService(ctx context.Context, log *zap.SugaredLogger, cfg *RedisConfiguration) (*RedisService, error) {
	rclient, err := redisClient(ctx, log, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create redis client: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	s := &RedisService{
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
func (s *RedisService) Close() error {
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
	id       string
	ctx      context.Context
	outCh    chan interface{}
	doneCh   chan error
	resultCh chan error
	topic    string

	// sendFn performs a select over outCh and the context, and returns true if
	// we sent the value, or false if the context fired.
	sendFn func(interface{}) (sent bool)
}

// redisClient returns a Redis client constructed from this process' environment
// variables.
func redisClient(ctx context.Context, log *zap.SugaredLogger, cfg *RedisConfiguration) (client *redis.Client, err error) {
	if cfg.Port == 0 {
		cfg.Port = 6379
	}

	log.Debugw("trying redis host", "host", cfg.Host, "port", cfg.Port)

	opts := DefaultRedisOpts
	opts.Addr = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	client = redis.NewClient(&opts).WithContext(ctx)

	if err := client.Ping().Err(); err != nil {
		_ = client.Close()
		log.Errorw("failed to ping redis host", "host", cfg.Host, "port", cfg.Port, "error", err)
		return nil, err
	}

	log.Debugw("redis ping OK", "opts", opts)
	return client, nil
}
