package util

import (
	"context"
	"reflect"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

func GetComms(ctx context.Context, key string, runenv *runtime.RunEnv) (*sync.Watcher, *sync.Writer, int64, error) {

	watcher, writer := sync.MustWatcherWriter(runenv)

	runenv.Message("Waiting for network initialization")
	if err := sync.WaitNetworkInitialized(ctx, runenv, watcher); err != nil {
		return watcher, writer, -1, err
	}
	runenv.Message("Network initilization complete")

	st := sync.Subtree{
		GroupKey:    key,
		PayloadType: reflect.TypeOf(""),
		KeyFunc: func(val interface{}) string {
			return val.(string)
		}}
	seq, err := writer.Write(&st, runenv.TestRun)
	runenv.Message("I have sequence ID %d\n", seq)
	return watcher, writer, seq, err
}
