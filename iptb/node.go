package iptb

import (
	"fmt"

	shell "github.com/ipfs/go-ipfs-api"
	testbedi "github.com/ipfs/iptb/testbed/interfaces"
	"github.com/multiformats/go-multiaddr"
)

type testNode struct {
	testbedi.Core
}

func (tn *testNode) URL() string {
	a, err := tn.APIAddr()
	if err != nil {
		panic(err)
	}

	addr, err := multiaddr.NewMultiaddr(a)
	if err != nil {
		return ""
	}

	ip, _ := addr.ValueForProtocol(multiaddr.P_IP4)
	port, _ := addr.ValueForProtocol(multiaddr.P_TCP)
	return fmt.Sprintf("%s:%s", ip, port)
}

// Client returns the http client to control this node.
func (tn *testNode) Client() *shell.Shell {
	return shell.NewShell(tn.URL())
}
