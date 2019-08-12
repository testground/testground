package cases

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/ipfs/test-pipeline"
	"github.com/ipfs/test-pipeline/iptb"
	"github.com/ipfs/test-pipeline/plans/smlbench"
)

// simpleAddGetTC is a simple test that adds a file of the specified size to an IPFS node, and tries to fetch it from
// another local node, by swarm connecting to the former first. It measures: time to add, time to connect, time to get.
type simpleAddGetTC struct {
	SizeBytes int64
}

var _ smlbench.SmallBenchmarksTestCase = (*simpleAddGetTC)(nil)

func (tc *simpleAddGetTC) Descriptor() *smlbench.TestCaseDescriptor {
	h := strings.ReplaceAll(strings.ToLower(humanize.IBytes(uint64(tc.SizeBytes))), " ", "")
	name := fmt.Sprintf("simple-add-get-%s", h)
	return &smlbench.TestCaseDescriptor{
		Name: name,
	}
}

func (tc *simpleAddGetTC) Configure(ctx context.Context, spec *iptb.TestEnsembleSpec) {
	spec.AddNodesDefaultConfig(iptb.NodeOpts{Initialize: true, Start: true}, "adder", "getter")
}

func (tc *simpleAddGetTC) Execute(ctx context.Context, ensemble *iptb.TestEnsemble) {
	adder := ensemble.GetNode("adder").Client()
	getter := ensemble.GetNode("getter").Client()

	// generate a random file of the designated size.
	file := smlbench.TempRandFile(ctx, ensemble.TempDir(), tc.SizeBytes)
	defer os.Remove(file.Name())

	tstarted := time.Now()
	cid, err := adder.Add(file)
	if err != nil {
		panic(err)
	}

	tpipeline.EmitMetric(ctx, smlbench.MetricTimeToAdd, float64(time.Now().Sub(tstarted)/time.Millisecond))

	addrs, err := ensemble.GetNode("adder").SwarmAddrs()
	if err != nil {
		panic(err)
	}

	tstarted = time.Now()
	err = getter.SwarmConnect(ctx, addrs[0])
	if err != nil {
		panic(err)
	}

	tpipeline.EmitMetric(ctx, smlbench.MetricTimeToConnect, float64(time.Now().Sub(tstarted)/time.Millisecond))

	tstarted = time.Now()
	err = getter.Get(cid, ensemble.TempDir())
	if err != nil {
		panic(err)
	}
	tpipeline.EmitMetric(ctx, smlbench.MetricTimeToGet, float64(time.Now().Sub(tstarted)/time.Millisecond))
}
