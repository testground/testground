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

var _ utils.SmallBenchmarksTestCase = (*SimpleAddTC)(nil)

func (tc *SimpleAddTC) Name() string {
	h := strings.ReplaceAll(strings.ToLower(humanize.IBytes(uint64(tc.SizeBytes))), " ", "")
	return fmt.Sprintf("simple-add-%s", h)
}

func (tc *SimpleAddTC) Configure(runenv *runtime.RunEnv, spec *iptb.TestEnsembleSpec) {
	spec.AddNodesDefaultConfig(iptb.NodeOpts{Initialize: true, Start: true}, "adder")
}

func (tc *SimpleAddTC) Execute(runenv *runtime.RunEnv, ensemble *iptb.TestEnsemble) error {
	node := ensemble.GetNode("adder")
	client := node.Client()

	filePath, err := runenv.CreateRandomFile(ensemble.TempDir(), tc.SizeBytes)
	if err != nil {
		return err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer os.Remove(filePath)

	tstarted := time.Now()
	_, err = client.Add(file)
	if err != nil {
		return err
	}

	runenv.EmitMetric(utils.MetricTimeToAdd, float64(time.Now().Sub(tstarted)/time.Millisecond))
	return nil
}
