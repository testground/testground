package test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dustin/go-humanize"

	utils "github.com/ipfs/testground/plans/smlbench/utils"
	"github.com/ipfs/testground/sdk/iptb"
	"github.com/ipfs/testground/sdk/runtime"
)

// simpleAddTC is a simple test that adds a file of the specified size to an IPFS node. It measures time to add.
type SimpleAddTC struct {
	SizeBytes int64
}

var _ utils.SmallBenchmarksTestCase = (*simpleAddTC)(nil)

func (tc *SimpleAddTC) Name() string {
	h := strings.ReplaceAll(strings.ToLower(humanize.IBytes(uint64(tc.SizeBytes))), " ", "")
	return fmt.Sprintf("simple-add-%s", h)
}

func (tc *SimpleAddTC) Configure(runenv *runtime.RunEnv, spec *iptb.TestEnsembleSpec) {
	spec.AddNodesDefaultConfig(iptb.NodeOpts{Initialize: true, Start: true}, "adder")
}

func (tc *SimpleAddTC) Execute(runenv *runtime.RunEnv, ensemble *iptb.TestEnsemble) {
	node := ensemble.GetNode("adder")
	client := node.Client()

	file := utils.TempRandFile(runenv, ensemble.TempDir(), tc.SizeBytes)
	defer os.Remove(file.Name())

	tstarted := time.Now()
	_, err := client.Add(file)
	if err != nil {
		runenv.Abort(err)
		return
	}

	runenv.EmitMetric(utils.MetricTimeToAdd, float64(time.Now().Sub(tstarted)/time.Millisecond))
	runenv.OK()
}
