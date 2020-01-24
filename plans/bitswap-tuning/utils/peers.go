package utils

import (
	"bytes"
	"context"
	"fmt"
	"time"

	host "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
)

func AddrInfosFromChan(peerCh chan *peer.AddrInfo, count int, timeout time.Duration) ([]peer.AddrInfo, error) {
	var ais []peer.AddrInfo
	for i := 1; i <= count; i++ {
		select {
		case ai := <-peerCh:
			ais = append(ais, *ai)

		case <-time.After(timeout):
			return nil, fmt.Errorf("no new peers in %d seconds", timeout/time.Second)
		}
	}
	return ais, nil
}

func DialOtherPeers(ctx context.Context, self host.Host, ais []peer.AddrInfo) ([]peer.AddrInfo, error) {
	// Grab list of other peers that are available for this Run
	var toDial []peer.AddrInfo
	for _, ai := range ais {
		id1, _ := ai.ID.MarshalBinary()
		id2, _ := self.ID().MarshalBinary()

		// skip over dialing ourselves, and prevent TCP simultaneous
		// connect (known to fail) by only dialing peers whose peer ID
		// is smaller than ours.
		if bytes.Compare(id1, id2) < 0 {
			toDial = append(toDial, ai)
		}
	}

	// Dial to all the other peers
	for _, ai := range toDial {
		if err := self.Connect(ctx, ai); err != nil {
			return nil, fmt.Errorf("Error while dialing peer %v: %w", ai.Addrs, err)
		}
	}

	return toDial, nil
}
