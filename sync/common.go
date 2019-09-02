package sync

import (
	"fmt"

	capi "github.com/hashicorp/consul/api"
	"github.com/ipfs/testground/api"
)

// ConsulClient returns a consul client from this processes environment
// variables, or panics if unable to create one.
func ConsulClient() (client *capi.Client) {
	client, err := capi.NewClient(capi.DefaultConfig())
	if err != nil {
		panic(err)
	}
	return client
}

// MustWatchWrite proxies to WatchWrite, panicking if an error occurs.
func MustWatchWrite(client *capi.Client, runenv *api.RunEnv) (*Watcher, *Writer) {
	watcher, writer, err := WatchWrite(client, runenv)
	if err != nil {
		panic(err)
	}
	return watcher, writer
}

// WatchWrite creates a Watcher and a Writer object associated with this test
// run's sync tree.
func WatchWrite(client *capi.Client, runenv *api.RunEnv) (*Watcher, *Writer, error) {
	watcher, err := WatchTree(client, runenv)
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
	prefix := fmt.Sprintf("run/%s/plan/%s/case/%s",
		runenv.TestRun,
		runenv.TestPlan,
		runenv.TestCase)

	return prefix
}
