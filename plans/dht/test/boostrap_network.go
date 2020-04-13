package test

import (
	"context"
	"github.com/ipfs/testground/sdk/runtime"
)

func BootstrapNetwork(runenv *runtime.RunEnv) error {
	commonOpts := GetCommonOpts(runenv)

	ctx, cancel := context.WithTimeout(context.Background(), commonOpts.Timeout)
	defer cancel()

	_, err := Base(ctx, runenv, commonOpts)
	if err != nil {
		return err
	}

	return nil
}
