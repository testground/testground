package test

import (
	"context"
	"github.com/testground/testground/sdk/runtime"
)

func All(runenv *runtime.RunEnv) error {
	commonOpts := GetCommonOpts(runenv)

	ctx, cancel := context.WithTimeout(context.Background(), commonOpts.Timeout)
	defer cancel()

	ri, err := Base(ctx, runenv, commonOpts)
	if err != nil {
		return err
	}

	if err := TestFindPeers(ctx, ri); err != nil {
		return err
	}
	if err := TestGetClosestPeers(ctx, ri); err != nil {
		return err
	}
	if err := TestProviderRecords(ctx, ri); err != nil {
		return err
	}
	if err := TestIPNSRecords(ctx, ri); err != nil {
		return err
	}

	Teardown(ctx, ri.RunInfo)
	return nil
}
