package testground

import (
	"fmt"

	"github.com/ipfs/testground/api"
	"github.com/ipfs/testground/plans/dht"
)

// Census contains all test plans known to the testground.
var Census = []api.TestPlan{
	&dht.TestPlan{},
}

// PrintTestCensus prints all test plans known to the testground.
func PrintTestCensus() {
	for _, tp := range Census {
		desc := tp.Descriptor()
		fmt.Println("test plan" + desc.Name + ":")
		for _, tc := range desc.TestCases {
			fmt.Println("\t" + tc)
		}
	}
}
