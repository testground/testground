package test

import (
	"context"
	"encoding/json"
	"os"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/libp2p/go-libp2p-core/routing"
)

func monitorEvents(ctx context.Context, runenv *runtime.RunEnv) (context.Context, error) {
	ctx, events := routing.RegisterForQueryEvents(ctx)
	f, err := os.OpenFile(runenv.TestRun+".json", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return nil, err
	}

	go func() {
		defer f.Close()

		for e := range events {
			bytes, err := json.Marshal(e)
			if err != nil {
				panic(err)
			}

			_, err = f.WriteString(string(bytes) + "\n")
			if err != nil {
				panic(err)
			}
		}
	}()

	return ctx, nil
}
