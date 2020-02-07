package utils

import (
	"context"
	"io"
	"strings"
	"time"

	bs "github.com/ipfs/go-bitswap"
	bsnet "github.com/ipfs/go-bitswap/network"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	delayed "github.com/ipfs/go-datastore/delayed"
	ds_sync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	chunker "github.com/ipfs/go-ipfs-chunker"
	delay "github.com/ipfs/go-ipfs-delay"
	files "github.com/ipfs/go-ipfs-files"
	nilrouting "github.com/ipfs/go-ipfs-routing/none"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	unixfile "github.com/ipfs/go-unixfs/file"
	"github.com/ipfs/go-unixfs/importer/balanced"
	"github.com/ipfs/go-unixfs/importer/helpers"
	"github.com/ipfs/go-unixfs/importer/trickle"
	"github.com/ipfs/testground/sdk/runtime"
	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core"
	"github.com/multiformats/go-multihash"
	"github.com/pkg/errors"
)

// Adapted from the netflix/p2plab repo under an Apache-2 license.
// Original source code located at https://github.com/Netflix/p2plab/blob/master/peer/peer.go
type Node struct {
	Host    core.Host
	Bitswap *bs.Bitswap
	Dserv   ipld.DAGService
}

func (n *Node) Close() {
	n.Bitswap.Close()
}

// CreateNode creates a libp2p Node with a Bitswap instance
func CreateNode(ctx context.Context, runenv *runtime.RunEnv) (*Node, error) {
	bstoreDelay := 0
	if runenv.IsParamSet("bstore_delay_ms") {
		bstoreDelay = runenv.IntParam("bstore_delay_ms")
	}

	h, err := libp2p.New(ctx)
	if err != nil {
		return nil, err
	}

	routing, err := nilrouting.ConstructNilRouting(context.Background(), nil, nil, nil)
	if err != nil {
		return nil, err
	}
	net := bsnet.NewFromIpfsHost(h, routing)

	bsdelay := delay.Fixed(time.Duration(bstoreDelay) * time.Millisecond)
	dstore := ds_sync.MutexWrap(delayed.New(ds.NewMapDatastore(), bsdelay))
	bstore, err := blockstore.CachedBlockstore(ctx,
		blockstore.NewBlockstore(ds_sync.MutexWrap(dstore)),
		blockstore.DefaultCacheOpts())
	if err != nil {
		return nil, err
	}

	bitswap := bs.New(ctx, net, bstore).(*bs.Bitswap)
	bserv := blockservice.New(bstore, bitswap)
	dserv := merkledag.NewDAGService(bserv)
	return &Node{h, bitswap, dserv}, nil
}

type AddSettings struct {
	Layout    string
	Chunker   string
	RawLeaves bool
	Hidden    bool
	NoCopy    bool
	HashFunc  string
	MaxLinks  int
}

func (n *Node) Add(ctx context.Context, r io.Reader) (ipld.Node, error) {
	settings := AddSettings{
		Layout:    "balanced",
		Chunker:   "size-262144",
		RawLeaves: false,
		Hidden:    false,
		NoCopy:    false,
		HashFunc:  "sha2-256",
		MaxLinks:  helpers.DefaultLinksPerBlock,
	}
	// for _, opt := range opts {
	// 	err := opt(&settings)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// }

	prefix, err := merkledag.PrefixForCidVersion(1)
	if err != nil {
		return nil, errors.Wrap(err, "unrecognized CID version")
	}

	hashFuncCode, ok := multihash.Names[strings.ToLower(settings.HashFunc)]
	if !ok {
		return nil, errors.Wrapf(err, "unrecognized hash function %q", settings.HashFunc)
	}
	prefix.MhType = hashFuncCode

	dbp := helpers.DagBuilderParams{
		Dagserv:    n.Dserv,
		RawLeaves:  settings.RawLeaves,
		Maxlinks:   settings.MaxLinks,
		NoCopy:     settings.NoCopy,
		CidBuilder: &prefix,
	}

	chnk, err := chunker.FromString(r, settings.Chunker)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create chunker")
	}

	dbh, err := dbp.New(chnk)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create dag builder")
	}

	var nd ipld.Node
	switch settings.Layout {
	case "trickle":
		nd, err = trickle.Layout(dbh)
	case "balanced":
		nd, err = balanced.Layout(dbh)
	default:
		return nil, errors.Errorf("unrecognized layout %q", settings.Layout)
	}

	return nd, err
}

func (n *Node) FetchGraph(ctx context.Context, c cid.Cid) error {
	ng := merkledag.NewSession(ctx, n.Dserv)
	return Walk(ctx, c, ng)
}

func (n *Node) Get(ctx context.Context, c cid.Cid) (files.Node, error) {
	nd, err := n.Dserv.Get(ctx, c)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get file %q", c)
	}

	return unixfile.NewUnixfsFile(ctx, n.Dserv, nd)
}
