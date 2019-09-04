package sync

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/ipfs/testground/api"
	"github.com/ipfs/testground/logging"

	"github.com/go-redis/redis"
)

// RedisClient returns a consul client from this processes environment
// variables, or panics if unable to create one.
func RedisClient() (client *redis.Client, err error) {
	// TODO: will need to populate opts from an env variable.
	opts := &redis.Options{}
	client = redis.NewClient(opts)

	// PING redis to make sure we're alive.
	err = client.Ping().Err()
	return client, err
}

// MustWatcherWriter proxies to WatcherWriter, panicking if an error occurs.
func MustWatcherWriter(client *redis.Client, runenv *api.RunEnv) (*Watcher, *Writer) {
	watcher, writer, err := WatcherWriter(client, runenv)
	if err != nil {
		panic(err)
	}
	return watcher, writer
}

// WatcherWriter creates a Watcher and a Writer object associated with this test
// run's sync tree.
func WatcherWriter(client *redis.Client, runenv *api.RunEnv) (*Watcher, *Writer, error) {
	watcher, err := NewWatcher(client, runenv)
	if err != nil {
		return nil, nil, err
	}

	writer, err := NewWriter(client, runenv)
	if err != nil {
		return nil, nil, err
	}

	return watcher, writer, nil
}

func basePrefix(runenv *api.RunEnv) string {
	p := fmt.Sprintf("run:%s:plan:%s:case:%s", runenv.TestRun, runenv.TestPlan, runenv.TestCase)
	return p
}

// mvccFromKey extracts the MVCC counter from the key. If the last token is not
// an MVCC int value, it panics.
func mvccFromKey(key string) int {
	splt := strings.Split(key, ":")
	mvcc, err := strconv.Atoi(splt[len(splt)-1])
	if err != nil {
		panic(err)
	}
	return mvcc
}

// decodePayload extracts a value of the specified type from incoming json, or
// panics if not possible.
func decodePayload(val interface{}, typ reflect.Type) reflect.Value {
	// Deserialize the value.
	payload := reflect.New(typ)
	raw, ok := val.(string)
	if !ok {
		panic("payload not a string")
	}
	if err := json.Unmarshal([]byte(raw), payload.Interface()); err != nil {
		logging.S().Warnw("failed to decode value", "data", string(raw), "type", typ)
		panic("failed to decode value")
	}
	return payload
}
