package smlbench

import "github.com/ipfs/testground"

var (
	MetricTimeToAdd     = &testground.MetricDefinition{Name: "time_to_add", Unit: "ms", ImprovementDir: -1}
	MetricTimeToConnect = &testground.MetricDefinition{Name: "time_to_connect", Unit: "ms", ImprovementDir: -1}
	MetricTimeToGet     = &testground.MetricDefinition{Name: "time_to_get", Unit: "ms", ImprovementDir: -1}
)
