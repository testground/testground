package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/sdk/runtime"

	"github.com/go-redis/redis/v7"
)

const (
	EnvRedisHost  = "REDIS_HOST"
	EnvRedisPort  = "REDIS_PORT"
	RedisHostname = "testground-redis"
	HostHostname  = "host.docker.internal"
)

// redisClient returns a consul client from this processes environment
// variables, or panics if unable to create one.
//
// NOTE: Canceling the context cancels the call to this function, it does not
// affect the returned client.
//
// TODO: source redis URL from environment variables. The Redis host and port
// will be wired in by Nomad/Swarm.
func redisClient(ctx context.Context, runenv *runtime.RunEnv) (client *redis.Client, err error) {
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
		//
		// TODO: Pick these fallbacks based on the runner.
		tryHosts = []string{RedisHostname, HostHostname, "localhost"}
	} else {
		tryHosts = []string{host}
	}

	// TODO: will need to populate opts from an env variable.
	opts := redis.Options{
		MaxRetries:      5,
		MinRetryBackoff: 1 * time.Second,
		MaxRetryBackoff: 3 * time.Second,
		DialTimeout:     10 * time.Second,
		ReadTimeout:     10 * time.Second,
	}

	for _, h := range tryHosts {
		logging.S().Debugw("resolving redis host", "host", h)

		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, h)
		if err != nil {
			logging.S().Debugw("failed to resolve redis host", "host", h, "error", err)
			continue
		}
		for _, addr := range addrs {
			logging.S().Debugw("trying redis host", "host", h, "address", addr, "error", err)
			opts := opts // copy to be safe.
			// Use TCPAddr to properly handle IPv6 addresses.
			opts.Addr = (&net.TCPAddr{IP: addr.IP, Zone: addr.Zone, Port: port}).String()
			client = redis.NewClient(&opts)

			// PING redis to make sure we're alive.
			if err := client.WithContext(ctx).Ping().Err(); err != nil {
				client.Close()
				logging.S().Debugw("failed to ping redis host", "host", h, "address", addr, "error", err)
				continue
			}

			logging.S().Debugw("redis options", "addr", opts.Addr)

			return client, nil
		}
	}
	return nil, fmt.Errorf("no viable redis host found")
}

// MustWatcherWriter proxies to WatcherWriter, panicking if an error occurs.
//
// NOTE: Canceling the context cancels the call to this function, it does not
// affect the returned watcher and writer.
func MustWatcherWriter(ctx context.Context, runenv *runtime.RunEnv) (*Watcher, *Writer) {
	watcher, writer, err := WatcherWriter(ctx, runenv)
	if err != nil {
		panic(err)
	}
	return watcher, writer
}

// WatcherWriter creates a Watcher and a Writer object associated with this test
// run's sync tree.
//
// NOTE: Canceling the context cancels the call to this function, it does not
// affect the returned watcher and writer.
func WatcherWriter(ctx context.Context, runenv *runtime.RunEnv) (*Watcher, *Writer, error) {
	watcher, err := NewWatcher(ctx, runenv)
	if err != nil {
		return nil, nil, err
	}

	writer, err := NewWriter(ctx, runenv)
	if err != nil {
		return nil, nil, err
	}

	return watcher, writer, nil
}

func basePrefix(runenv *runtime.RunEnv) string {
	p := fmt.Sprintf("run:%s:plan:%s:case:%s", runenv.TestRun, runenv.TestPlan, runenv.TestCase)
	return p
}

// decodePayload extracts a value of the specified type from incoming json.
func decodePayload(val interface{}, typ reflect.Type) (reflect.Value, error) {
	// Deserialize the value.
	payload := reflect.New(typ)
	raw, ok := val.(string)
	if !ok {
		panic("payload not a string")
	}
	if err := json.Unmarshal([]byte(raw), payload.Interface()); err != nil {
		return reflect.Value{}, fmt.Errorf("failed to decode as type %s: %s", typ, string(raw))
	}
	return payload, nil
}
