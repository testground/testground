package test

import (
	"context"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

func Barrier(runenv *runtime.RunEnv) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 360)
	defer cancel()

	watcher, writer := sync.MustWatcherWriter(ctx, runenv)
	defer watcher.Close()
	defer writer.Close()

	err := SetupNetwork(ctx, runenv, watcher, writer)
	if err != nil {
		return err
	}

	return nil
}
