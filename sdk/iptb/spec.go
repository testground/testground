package iptb

import (
	"fmt"

	config "github.com/ipfs/go-ipfs-config"
)

type TestEnsembleSpec struct {
	tags map[string]NodeOpts
}

type AddRepoOptions func(*config.Config) error

type NodeOpts struct {
	// Initialize tells us to initialize the newly created IPFS nodes (ipfs init).
	Initialize bool
	// Start tells us to start the newly created IPFS nodes as daemons.
	Start bool
	// AddRepoOptions is a function that modifies the configuration of a repository.
	AddRepoOptions AddRepoOptions
}

// NewTestEnsembleSpec creates a blank specification for a test ensemble.
func NewTestEnsembleSpec() *TestEnsembleSpec {
	return &TestEnsembleSpec{
		tags: make(map[string]NodeOpts),
	}
}

// AddNodesDefaultConfig adds as many nodes to the ensemble as tags provided, making sure that tags are unique.
// By default, all nodes will use the default IPFS configuration, but you can pass a AddOptions function on the
// Node Options to replace some of the values.
func (tes *TestEnsembleSpec) AddNodesDefaultConfig(opts NodeOpts, tags ...string) {
	for _, tag := range tags {
		if _, ok := tes.tags[tag]; ok {
			panic(fmt.Sprintf("tag %s already exists in the test ensemble", tag))
		}

		tes.tags[tag] = opts
	}
}
