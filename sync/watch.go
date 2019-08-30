package sync

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"sync"

	"github.com/ipfs/testground/api"
	"github.com/ipfs/testground/logging"

	capi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

type Action int

const (
	ActionAdded Action = iota
	ActionModified
	ActionDeleted
)

var (
	regexPeers = regexp.MustCompile("\\/nodes\\/?P<peerId>(.*?)/addrs")
)

type Event struct {
	Action Action
}

type PeerEvent struct {
	Event

	AddrInfo *peer.AddrInfo
}

type watcher struct {
	lk     sync.RWMutex
	client *capi.Client
	plan   *watch.Plan
	chs    struct {
		peers map[chan *PeerEvent]struct{}
	}
}

type sink struct {
	regex *regexp.Regexp
	out   <-chan struct{}
}

func (w *watcher) SubscribePeers(ch chan *PeerEvent) (cancel func(), err error) {
	w.lk.Lock()
	defer w.lk.Unlock()

	w.chs.peers[ch] = struct{}{}
	cancel = func() {
		w.lk.Lock()
		defer w.lk.Unlock()
		delete(w.chs.peers, ch)
	}
	return cancel, nil
}

func (w *watcher) route(index uint64, v interface{}) {
	kvs, ok := v.(capi.KVPairs)
	if !ok {
		logging.S().Warn("watcher received unexpected type")
		return
	}

	w.lk.RLock()
	defer w.lk.RUnlock()

	for _, kv := range kvs {
		match := regexPeers.FindStringSubmatch(kv.Key)
		if len(match) > 0 {
			w.handlePeer(kv, match)
		}
	}
}

func (w *watcher) handlePeer(kv *capi.KVPair, match []string) (handled bool) {
	if len(w.chs.peers) == 0 {
		return true
	}

	id, err := peer.IDB58Decode(match[1])
	if err != nil {
		logging.S().Warnw("failed to decode peer ID", "data", match[1])
		return false
	}

	var addrs []string
	if err = json.Unmarshal(kv.Value, &addrs); err != nil {
		logging.S().Warnw("failed to decode addresses", "data", string(kv.Value))
		return false
	}

	maddrs := make([]multiaddr.Multiaddr, 0, len(addrs))
	for _, a := range addrs {
		ma, err := multiaddr.NewMultiaddr(a)
		if err != nil {
			logging.S().Warnw("failed to decode multiaddr", "data", string(kv.Value))
			return false
		}
		maddrs = append(maddrs, ma)
	}

	ai := &peer.AddrInfo{
		ID:    id,
		Addrs: maddrs,
	}

	action := ActionAdded
	v, _, err := w.client.KV().Get(kv.Key, nil)
	if err != nil {
		logging.S().Warnw("failed to query the status of the key", "data", string(kv.Key), "error", err)
		return false
	}
	if v == nil {
		action = ActionDeleted
	}

	for ch := range w.chs.peers {
		pe := &PeerEvent{AddrInfo: ai, Event: Event{Action: action}}
		ch <- pe
	}

	return true
}

func (w *watcher) Close() error {
	w.lk.Lock()
	defer w.lk.Unlock()

	w.plan.Stop()

	for ch := range w.chs.peers {
		close(ch)
		delete(w.chs.peers, ch)
	}

	return nil
}

func WatchTree(client *capi.Client, runenv *api.RunEnv) (*watcher, error) {
	prefix := fmt.Sprintf("run/%s/plan/%s/case/%s", runenv.TestRun, runenv.TestPlan, runenv.TestCase)

	plan, err := watch.Parse(map[string]interface{}{
		"type":   "keyprefix",
		"prefix": prefix,
	})

	if err != nil {
		return nil, err
	}

	res := &watcher{
		client: client,
		plan:   plan,
	}

	plan.Handler = res.route

	log := log.New(os.Stdout, "watch", log.LstdFlags)
	go plan.RunWithClientAndLogger(client, log)

	return res, nil
}
