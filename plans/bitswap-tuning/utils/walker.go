package utils

import (
	"context"

	cid "github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	"golang.org/x/sync/errgroup"
)

// Adapted from the netflix/p2plab repo under an Apache-2 license.
// Original source code located at https://github.com/Netflix/p2plab/blob/master/dag/walker.go
func Walk(ctx context.Context, c cid.Cid, ng ipld.NodeGetter) error {
	nd, err := ng.Get(ctx, c)
	if err != nil {
		return err
	}

	return walk(ctx, nd, ng)
}

func walk(ctx context.Context, nd ipld.Node, ng ipld.NodeGetter) error {
	var cids []cid.Cid
	for _, link := range nd.Links() {
		cids = append(cids, link.Cid)
	}

	eg, gctx := errgroup.WithContext(ctx)

	ndChan := ng.GetMany(ctx, cids)
	for ndOpt := range ndChan {
		if ndOpt.Err != nil {
			return ndOpt.Err
		}

		nd := ndOpt.Node
		eg.Go(func() error {
			return walk(gctx, nd, ng)
		})
	}

	err := eg.Wait()
	if err != nil {
		return err
	}

	return nil
}
