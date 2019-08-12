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

// simpleAddTC is a simple test that adds a file of the specified size to an IPFS node. It measures time to add.
type simpleAddTC struct {
	SizeBytes int64
}

var _ smlbench.SmallBenchmarksTestCase = (*simpleAddTC)(nil)

func (tc *simpleAddTC) Descriptor() *smlbench.TestCaseDescriptor {
	h := strings.ReplaceAll(strings.ToLower(humanize.IBytes(uint64(tc.SizeBytes))), " ", "")
	name := fmt.Sprintf("simple-add-%s", h)
	return &smlbench.TestCaseDescriptor{
		Name: name,
	}
}

func (tc *simpleAddTC) Configure(ctx context.Context, spec *iptb.TestEnsembleSpec) {
	spec.AddNodesDefaultConfig(iptb.NodeOpts{Initialize: true, Start: true}, "adder")
}

func (tc *simpleAddTC) Execute(ctx context.Context, ensemble *iptb.TestEnsemble) {
	node := ensemble.GetNode("adder")
	client := node.Client()

	file := smlbench.TempRandFile(ctx, ensemble.TempDir(), tc.SizeBytes)
	defer os.Remove(file.Name())

	tstarted := time.Now()
	_, err := client.Add(file)
	if err != nil {
		panic(err)
	}

	tpipeline.EmitMetric(ctx, smlbench.MetricTimeToAdd, float64(time.Now().Sub(tstarted)/time.Millisecond))
}
