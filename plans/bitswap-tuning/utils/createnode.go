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
	core "github.com/libp2p/go-libp2p-core"
	"github.com/multiformats/go-multihash"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type NodeType int

const (
	// Seeds data
	Seed NodeType = iota
	// Fetches data from seeds
	Leech
	// Doesn't seed or fetch data
	Passive
)

func (nt NodeType) String() string {
	return [...]string{"Seed", "Leech", "Passive"}[nt]
}

// Adapted from the netflix/p2plab repo under an Apache-2 license.
// Original source code located at https://github.com/Netflix/p2plab/blob/master/peer/peer.go
type Node struct {
	Bitswap *bs.Bitswap
	Dserv   ipld.DAGService
}

func (n *Node) Close() error {
	return n.Bitswap.Close()
}

func CreateBlockstore(ctx context.Context, bstoreDelay time.Duration) (blockstore.Blockstore, error) {
	bsdelay := delay.Fixed(bstoreDelay)
	dstore := ds_sync.MutexWrap(delayed.New(ds.NewMapDatastore(), bsdelay))
	return blockstore.CachedBlockstore(ctx,
		blockstore.NewBlockstore(ds_sync.MutexWrap(dstore)),
		blockstore.DefaultCacheOpts())
}

func ClearBlockstore(ctx context.Context, bstore blockstore.Blockstore) error {
	ks, err := bstore.AllKeysChan(ctx)
	if err != nil {
		return err
	}
	g := errgroup.Group{}
	for k := range ks {
		c := k
		g.Go(func() error {
			return bstore.DeleteBlock(c)
		})
	}
	return g.Wait()
}

func CreateBitswapNode(ctx context.Context, h core.Host, bstore blockstore.Blockstore) (*Node, error) {
	routing, err := nilrouting.ConstructNilRouting(ctx, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	net := bsnet.NewFromIpfsHost(h, routing)
	bitswap := bs.New(ctx, net, bstore).(*bs.Bitswap)
	bserv := blockservice.New(bstore, bitswap)
	dserv := merkledag.NewDAGService(bserv)
	return &Node{bitswap, dserv}, nil
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
