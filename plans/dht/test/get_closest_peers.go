package test

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/ipfs/testground/plans/dht/utils"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ipfs/go-cid"
	u "github.com/ipfs/go-ipfs-util"

	"github.com/libp2p/go-libp2p-core/peer"
	kbucket "github.com/libp2p/go-libp2p-kbucket"

	"github.com/ipfs/testground/sdk/runtime"
)

func GetClosestPeers(runenv *runtime.RunEnv) error {
	commonOpts := GetCommonOpts(runenv)

	ctx, cancel := context.WithTimeout(context.Background(), commonOpts.Timeout)
	defer cancel()

	ri, err := Base(ctx, runenv, commonOpts)
	if err != nil {
		return err
	}

	if err := TestGetClosestPeers(ctx, ri); err != nil {
		return err
	}
	Teardown(ctx, ri.RunInfo)

	return nil
}

func TestGetClosestPeers(ctx context.Context, ri *DHTRunInfo) error {
	fpOpts := getFindProvsParams(ri.RunEnv.RunParams.TestInstanceParams)
	runenv := ri.RunEnv

	// TODO: This is hacky we should probably thread through a separate GCPRecordCount variable
	maxRecCount := 0
	for _, g := range ri.GroupProperties {
		gOpts := getFindProvsParams(g.Params)
		if gOpts.RecordCount > maxRecCount {
			maxRecCount = gOpts.RecordCount
		}
	}

	// Calculate the CIDs we're dealing with.
	cids := func() (out []cid.Cid) {
		for i := 0; i < maxRecCount; i++ {
			c := fmt.Sprintf("CID %d - seeded with %d", i, fpOpts.RecordSeed)
			out = append(out, cid.NewCidV0(u.Hash([]byte(c))))
		}
		return out
	}()

	node := ri.Node
	others := ri.Others

	stager := utils.NewBatchStager(ctx, node.info.Seq, runenv.TestInstanceCount, "get-closest-peers", ri.RunInfo)
	if err := stager.Begin(); err != nil {
		return err
	}

	runenv.RecordMessage("start gcp loop")
	if fpOpts.SearchRecords {
		g := errgroup.Group{}
		for index, cid := range cids {
			i := index
			c := cid
			g.Go(func() error {
				p := peer.ID(c.Bytes())
				ectx, cancel := context.WithCancel(ctx)
				ectx = TraceQuery(ctx, runenv, node, p.Pretty(), "get-closest-peers")
				t := time.Now()
				pids, err := node.dht.GetClosestPeers(ectx, c.KeyString())
				cancel()

				peers := make([]peer.ID, 0, node.info.Properties.BucketSize)
				for p := range pids {
					peers = append(peers, p)
				}

				if err == nil {
					runenv.RecordMetric(&runtime.MetricDefinition{
						Name:           fmt.Sprintf("time-to-gcp-%d", i),
						Unit:           "ns",
						ImprovementDir: -1,
					}, float64(time.Since(t).Nanoseconds()))

					runenv.RecordMetric(&runtime.MetricDefinition{
						Name:           fmt.Sprintf("gcp-peers-found-%d", i),
						Unit:           "peers",
						ImprovementDir: 1,
					}, float64(len(pids)))

					actualClosest := getClosestPeerRanking(node, others, c)
					outputGCP(runenv, node.info.Addrs.ID, c, peers, actualClosest)
				} else {
					runenv.RecordMessage("Error during GCP %w", err)
				}
				return err
			})
		}

		if err := g.Wait(); err != nil {
			_ = stager.End()
			return fmt.Errorf("failed while finding providerss: %s", err)
		}
	}

	runenv.RecordMessage("done gcp loop")

	if err := stager.End(); err != nil {
		return err
	}

	return nil
}

func getClosestPeerRanking(me *NodeParams, others map[peer.ID]*DHTNodeInfo, target cid.Cid) []peer.ID {
	var allPeers []peer.ID
	allPeers = append(allPeers, me.dht.PeerID())
	for p := range others {
		allPeers = append(allPeers, p)
	}

	kadTarget := kbucket.ConvertKey(target.KeyString())
	return kbucket.SortClosestPeers(allPeers, kadTarget)
}

func outputGCP(runenv *runtime.RunEnv, me peer.ID, target cid.Cid, peers, rankedPeers []peer.ID) {
	peerStrs := make([]string, len(peers))
	kadPeerStrs := make([]string, len(peers))

	for i, p := range peers {
		peerStrs[i] = p.String()
		kadPeerStrs[i] = hex.EncodeToString(kbucket.ConvertKey(string(p)))
	}

	actualClosest := rankedPeers[:len(peers)]

	nodeLogger.Infow("gcp-results",
		"me", me.String(),
		"KadMe", kbucket.ConvertKey(string(me)),
		"target", target,
		"peers", peers,
		"actual", actualClosest,
		"KadTarget", kbucket.ConvertKey(target.KeyString()),
		"KadPeers", peerIDsToKadIDs(peers),
		"KadActual", peerIDsToKadIDs(actualClosest),
		"Scores", gcpScore(peers, rankedPeers),
	)

	nodeLogger.Sync()
}

func gcpScore(peers, rankedPeers []peer.ID) []int {
	getIndex := func(peers []peer.ID, target peer.ID) int {
		for i, p := range peers {
			if p == target {
				return i
			}
		}
		return -1
	}

	// score is distance between actual ranking and our ranking
	var scores []int
	for i, p := range peers {
		diff := getIndex(rankedPeers, p) - i
		scores = append(scores, diff)
	}
	return scores
}

func peerIDsToKadIDs(peers []peer.ID) []kbucket.ID {
	kadIDs := make([]kbucket.ID, len(peers))
	for i, p := range peers {
		kadIDs[i] = kbucket.ConvertPeerID(p)
	}
	return kadIDs
}
