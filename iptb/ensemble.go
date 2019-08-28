package iptb

import (
	"context"
	"io/ioutil"
	"os"
	"sync"

	"github.com/ipfs/iptb/testbed"
	testbedi "github.com/ipfs/iptb/testbed/interfaces"
	tpipeline "github.com/ipfs/testground"
)

type TestEnsemble struct {
	ctx context.Context

	spec    *TestEnsembleSpec
	testbed *testbed.BasicTestbed
	tags    map[string]*testNode
}

// NewTestEnsemble creates a new test ensemble from a specification.
func NewTestEnsemble(ctx context.Context, spec *TestEnsembleSpec) *TestEnsemble {
	return &TestEnsemble{
		ctx:  ctx,
		spec: spec,
		tags: make(map[string]*testNode),
	}
}

// Initialize initializes the ensemble by materializing the IPTB testbed and associating nodes with tags.
func (te *TestEnsemble) Initialize() {
	var (
		tags  = te.spec.tags
		count = len(tags)
		tctx  = tpipeline.ExtractTestContext(te.ctx)
	)

	// temp dir, prefixed with the test plan name for debugging purposes.
	dir, err := ioutil.TempDir("", tctx.TestPlan)
	if err != nil {
		panic(err)
	}

	tb := testbed.NewTestbed(dir)
	te.testbed = &tb

	specs, err := testbed.BuildSpecs(tb.Dir(), count, "localipfs", nil)
	if err != nil {
		panic(err)
	}

	if err := testbed.WriteNodeSpecs(tb.Dir(), specs); err != nil {
		panic(err)
	}

	nodes, err := tb.Nodes()
	if err != nil {
		panic(err)
	}

	// assign IPFS nodes to tagged nodes; initialize and start if options tell us to.
	var (
		wg sync.WaitGroup
		i  int
	)
	for k, tn := range tags {
		node := &testNode{nodes[i]}
		te.tags[k] = node

		if tn.opts.Initialize || tn.opts.Start {
			wg.Add(1)

			go func(n testbedi.Core) {
				defer wg.Done()

				var err error
				if tn.opts.Initialize {
					_, err = n.Init(context.Background())
				}
				if err == nil && tn.opts.Start {
					_, err = n.Start(context.Background(), true)
				}
				if err != nil {
					panic(err)
				}
			}(node)
		}
		i++
	}
	wg.Wait()
}

// Destroy destroys the ensemble and cleans up its home directory.
func (te *TestEnsemble) Destroy() {
	nodes, err := te.testbed.Nodes()
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	for _, n := range nodes {
		wg.Add(1)
		go func(node testbedi.Core) {
			wg.Done()

			if err := n.Stop(te.ctx); err != nil {
				panic(err)
			}
		}(n)
	}
	wg.Wait()

	err = os.RemoveAll(te.testbed.Dir())
	if err != nil {
		panic(err)
	}
}

// Context returns the test context associated with this ensemble.
func (te *TestEnsemble) Context() context.Context {
	return te.ctx
}

// GetNode returns the node associated with this tag.
func (te *TestEnsemble) GetNode(tag string) *testNode {
	return te.tags[tag]
}

// TempDir
func (te *TestEnsemble) TempDir() string {
	return te.testbed.Dir()
}
