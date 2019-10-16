package iptb

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"sync"
	"time"

	"github.com/ipfs/testground/sdk/runtime"

	"github.com/ipfs/iptb/testbed"
	testbedi "github.com/ipfs/iptb/testbed/interfaces"
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
		tags   = te.spec.tags
		count  = len(tags)
		runenv = runtime.CurrentRunEnv()
	)

	// temp dir, prefixed with the test plan name for debugging purposes.
	dir, err := ioutil.TempDir("", runenv.TestPlan)
	if err != nil {
		panic(err)
	}

	tb := testbed.NewTestbed(dir)
	te.testbed = &tb

	attrs := map[string]string{}
	swarmaddrs := make([]string, 0)

	iface, err := net.InterfaceByName("eth0")
	if err == nil {
		addrs, err := iface.Addrs()
		if err != nil {
			panic(err)
		}
		for _, a := range addrs {
		    switch v := a.(type) {
		    case *net.IPAddr:
			// fmt.Printf("%v : %s (%s)\n", iface.Name, v, v.IP.DefaultMask())
		    case *net.IPNet:
			// fmt.Printf("%v : %s (%s)\n", iface.Name, v, v.IP.DefaultMask())
			if (v.IP.To4() != nil) {
				swarmaddrs = append(swarmaddrs, fmt.Sprintf("/ip4/%v/tcp/0", v.IP))
			}
		    }
		}
	}
	if len(swarmaddrs) > 0 {
		attrs["swarmaddr"] = swarmaddrs[0]
	}

	specs, err := testbed.BuildSpecs(tb.Dir(), count, "localipfs", attrs)
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
	for _, node := range nodes {
		wg.Add(1)
		go func(n testbedi.Core) {
			wg.Done()

			if err := n.Stop(te.ctx); err != nil {
				panic(err)
			}
		}(node)
	}
	wg.Wait()
	time.Sleep(100 * time.Millisecond) // Race?

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
