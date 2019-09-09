package iptb

import (
	"fmt"

	config "github.com/ipfs/go-ipfs-config"
)

type TestEnsembleSpec struct {
	tags map[string]*testNodeSpec
}

type NodeOpts struct {
	// Initialize tells us to initialize the newly created IPFS nodes (ipfs init).
	Initialize bool
	// Start tells us to start the newly created IPFS nodes as daemons.
	Start bool
}

type testNodeSpec struct {
	config *config.Config
	opts   NodeOpts
}

// NewTestEnsembleSpec creates a blank specification for a test ensemble.
func NewTestEnsembleSpec() *TestEnsembleSpec {
	return &TestEnsembleSpec{
		tags: make(map[string]*testNodeSpec),
	}
}

// AddNodesDefaultConfig adds as many nodes to the ensemble as tags provided, making sure that tags are unique.
//
// All nodes will use the default IPFS configuration.
//
// TODO: this method will be refactored once we add first-class support for configs;
//  configs may be provided via Options.
func (tes *TestEnsembleSpec) AddNodesDefaultConfig(opts NodeOpts, tags ...string) {
	for _, tag := range tags {
		if _, ok := tes.tags[tag]; ok {
			panic(fmt.Sprintf("tag %s already exists in the test ensemble", tag))
		}
		tn := &testNodeSpec{opts: opts}
		tes.tags[tag] = tn
	}
}
