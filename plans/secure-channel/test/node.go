package test

import (
	"context"
	csms "github.com/libp2p/go-conn-security-multistream"
	blankhost "github.com/libp2p/go-libp2p-blankhost"
	"github.com/libp2p/go-libp2p-core/crypto"
	host "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/mux"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/sec"
	"github.com/libp2p/go-libp2p-core/sec/insecure"
	mplex "github.com/libp2p/go-libp2p-mplex"
	noise "github.com/libp2p/go-libp2p-noise"
	"github.com/libp2p/go-libp2p-peerstore/pstoremem"
	secio "github.com/libp2p/go-libp2p-secio"
	swarm "github.com/libp2p/go-libp2p-swarm"
	tls "github.com/libp2p/go-libp2p-tls"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	msmux "github.com/libp2p/go-stream-muxer-multistream"
	"github.com/libp2p/go-tcp-transport"
	"github.com/multiformats/go-multiaddr"
)

func newHost(ctx context.Context, channelName string, key crypto.PrivKey) (host.Host, error) {
	pid, err := peer.IDFromPrivateKey(key)
	if err != nil {
		return nil, err
	}
	ps := pstoremem.NewPeerstore()
	err = ps.AddPrivKey(pid, key)
	if err != nil {
		return nil, err
	}
	err = ps.AddPubKey(pid, key.GetPublic())
	if err != nil {
		return nil, err
	}

	n := swarm.NewSwarm(ctx, pid, ps, nil) // TODO: add metrics reporter?

	h := blankhost.NewBlankHost(n)

	upgrader := new(tptu.Upgrader)
	upgrader.Secure, err = makeSecurityTransport(channelName, key)
	if err != nil {
		h.Close()
		return nil, err
	}
	upgrader.Muxer = makeMuxer()
	tcpTpt := tcp.NewTCPTransport(upgrader)
	err = n.AddTransport(tcpTpt)
	if err != nil {
		h.Close()
		return nil, err
	}

	listenAddrs := []multiaddr.Multiaddr{
		multiaddr.StringCast("/ip4/0.0.0.0/tcp/0"),
		multiaddr.StringCast("/ip6/::/tcp/0"),
	}
	if err := h.Network().Listen(listenAddrs...); err != nil {
		h.Close()
		return nil, err
	}

	return h, err
}

func makeSecurityTransport(name string, key crypto.PrivKey) (sec.SecureTransport, error) {
	secMuxer := new(csms.SSMuxer)

	switch name {
	case "none":
		id, err := peer.IDFromPrivateKey(key)
		if err != nil {
			return nil, err
		}
		tpt := insecure.NewWithIdentity(id, key)
		secMuxer.AddTransport(insecure.ID, tpt)
	case "secio":
		tpt, err := secio.New(key)
		if err != nil {
			return nil, err
		}
		secMuxer.AddTransport(secio.ID, tpt)
	case "tls":
		tpt, err := tls.New(key)
		if err != nil {
			return nil, err
		}
		secMuxer.AddTransport(tls.ID, tpt)
	case "noise":
		tpt, err := noise.New(key)
		if err != nil {
			return nil, err
		}
		secMuxer.AddTransport(noise.ID, tpt)
	default:
		panic("unknown secure_channel option " + name)

	}
	return secMuxer, nil
}

func makeMuxer() mux.Multiplexer {
	muxMuxer := msmux.NewBlankTransport()
	muxMuxer.AddTransport("/mplex/6.7.0", mplex.DefaultTransport)
	return muxMuxer
}

