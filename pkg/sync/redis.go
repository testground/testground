package sync

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v7"
	"go.uber.org/zap"
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
