package sync

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/testground/testground/sdk/runtime"

	"github.com/go-redis/redis/v7"
	"go.uber.org/zap"
)

const (
	RedisPayloadKey = "p"

	EnvRedisHost  = "REDIS_HOST"
	EnvRedisPort  = "REDIS_PORT"
	RedisHostname = "testground-redis"
	HostHostname  = "host.docker.internal"
)

// ErrNoRunParameters is returned by the generic client when an unbound context
// is passed in. See WithRunParams to bind RunParams to the context.
var ErrNoRunParameters = fmt.Errorf("no run parameters provided")

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

type Client struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	log       *zap.SugaredLogger
	extractor func(ctx context.Context) (rp *runtime.RunParams)

	rclient *redis.Client

	barrierCh chan *newBarrier
	newSubCh  chan *newSubscription
}

// NewBoundClient returns a new sync Client that is bound to the provided
// RunEnv. All operations will be automatically scoped to the keyspace of that
// run.
//
// The context passed in here will govern the lifecycle of the client.
// Cancelling it will cancel all ongoing operations. However, for a clean
// closure, the user should call Close().
//
// For test plans, a suitable context to pass here is the background context.
func NewBoundClient(ctx context.Context, runenv *runtime.RunEnv) (*Client, error) {
	return newClient(ctx, runenv.SLogger(), func(ctx context.Context) *runtime.RunParams {
		return &runenv.RunParams
	})
}

// MustBoundClient creates a new bound client by calling NewBoundClient, and
// panicking if it errors.
func MustBoundClient(ctx context.Context, runenv *runtime.RunEnv) *Client {
	c, err := NewBoundClient(ctx, runenv)
	if err != nil {
		panic(err)
	}
	return c
}

// NewGenericClient returns a new sync Client that is bound to no RunEnv.
// It is intended to be used by testground services like the sidecar.
//
// All operations expect to find the RunParams of the run to scope its actions
// inside the supplied context.Context. Call WithRunParams to bind the
// appropriate RunParams.
//
// The context passed in here will govern the lifecycle of the client.
// Cancelling it will cancel all ongoing operations. However, for a clean
// closure, the user should call Close().
//
// A suitable context to pass here is the background context of the main
// process.
func NewGenericClient(ctx context.Context, log *zap.SugaredLogger) (*Client, error) {
	return newClient(ctx, log, GetRunParams)
}

// MustGenericClient creates a new generic client by calling NewGenericClient,
// and panicking if it errors.
func MustGenericClient(ctx context.Context, log *zap.SugaredLogger) *Client {
	c, err := NewGenericClient(ctx, log)
	if err != nil {
		panic(err)
	}
	return c
}

// newClient creates a new sync client.
func newClient(ctx context.Context, log *zap.SugaredLogger, extractor func(ctx context.Context) *runtime.RunParams) (*Client, error) {
	rclient, err := redisClient(ctx, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create redis client: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	c := &Client{
		ctx:       ctx,
		cancel:    cancel,
		log:       log,
		extractor: extractor,
		rclient:   rclient,
		barrierCh: make(chan *newBarrier),
		newSubCh:  make(chan *newSubscription),
	}

	c.wg.Add(2)
	go c.barrierWorker()
	go c.subscriptionWorker()

	if debug := log.Desugar().Core().Enabled(zap.DebugLevel); debug {
		go func() {
			tick := time.NewTicker(1 * time.Second)
			defer tick.Stop()

			for {
				select {
				case <-tick.C:
					stats := rclient.PoolStats()
					log.Debugw("redis pool stats", "stats", stats)
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	return c, nil
}

// Close closes this client, cancels ongoing operations, and releases resources.
func (c *Client) Close() error {
	c.cancel()
	c.wg.Wait()

	return c.rclient.Close()
}

// newSubscription is an ancillary type used when creating a new Subscription.
type newSubscription struct {
	sub      *Subscription
	resultCh chan error
}

// newBarrier is an ancillary type used when creating a new Barrier.
type newBarrier struct {
	barrier  *Barrier
	resultCh chan error
}

// redisClient returns a Redis client constructed from this process' environment
// variables.
func redisClient(ctx context.Context, log *zap.SugaredLogger) (client *redis.Client, err error) {
	var (
		port = 6379
		host = os.Getenv(EnvRedisHost)
	)

	if portStr := os.Getenv(EnvRedisPort); portStr != "" {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse port '%q': %w", portStr, err)
		}
	}

	var tryHosts []string
	if host == "" {
		// Try to resolve the "testground-redis" host from Docker's DNS.
		//
		// Fall back to attempting to use `host.docker.internal` which
		// is only available in macOS and Windows.
		// Finally, falling back on localhost (for local:exec)
		tryHosts = []string{RedisHostname, HostHostname, "localhost"}
	} else {
		tryHosts = []string{host}
	}

	for _, h := range tryHosts {
		log.Debugw("resolving redis host", "host", h)

		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, h)
		if err != nil {
			log.Debugw("failed to resolve redis host", "host", h, "error", err)
			continue
		}
		for _, addr := range addrs {
			log.Debugw("trying redis host", "host", h, "address", addr, "error", err)
			opts := DefaultRedisOpts // copy to be safe.
			// Use TCPAddr to properly handle IPv6 addresses.
			opts.Addr = (&net.TCPAddr{IP: addr.IP, Zone: addr.Zone, Port: port}).String()
			client = redis.NewClient(&opts).WithContext(ctx)

			// PING redis to make sure we're alive.
			if err := client.Ping().Err(); err != nil {
				_ = client.Close()
				log.Debugw("failed to ping redis host", "host", h, "address", addr, "error", err)
				continue
			}

			log.Debugw("redis ping OK", "opts", opts)

			return client, nil
		}
	}
	return nil, fmt.Errorf("no viable redis host found")
}
