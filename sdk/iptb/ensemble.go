package iptb

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/ipfs/testground/sdk/runtime"

	config "github.com/ipfs/go-ipfs-config"
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

		if tn.Initialize || tn.Start {
			wg.Add(1)

			go func(n testbedi.Core) {
				defer wg.Done()

				var err error
				if tn.Initialize {
					_, err = n.Init(context.Background())

					// If we have a function to modify the options of the repository,
					// we load the current configuration, then we apply our custom function,
					// create a temporary file with the new configuration and finally
					// execute 'ipfs config replace <newConfigFile>'.
					if tn.AddRepoOptions != nil {
						cfg, err := getCurrentConfiguration(n)
						if err != nil {
							panic(err)
						}

						err = tn.AddRepoOptions(cfg)
						if err != nil {
							panic(err)
						}

						filePath, err := createConfigurationFile(cfg)
						if err != nil {
							panic(err)
						}
						defer os.Remove(filePath)

						_, err = n.RunCmd(context.Background(), nil, "ipfs", "config", "replace", filePath)
						if err != nil {
							panic(err)
						}
					}
				}

				if err == nil && tn.Start {
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

func getCurrentConfiguration(node testbedi.Core) (*config.Config, error) {
	out, err := node.RunCmd(context.Background(), nil, "ipfs", "config", "show")
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(out.Stdout())

	cfg := &config.Config{}
	return cfg, json.Unmarshal(buf.Bytes(), cfg)
}

func createConfigurationFile(cfg *config.Config) (string, error) {
	bytes, err := config.Marshal(cfg)
	if err != nil {
		return "", err
	}

	file, err := ioutil.TempFile("", "cfg")
	if err != nil {
		return "", err
	}
	defer file.Close()

	_, err = file.Write(bytes)
	if err != nil {
		return "", err
	}

	return file.Name(), nil
}
