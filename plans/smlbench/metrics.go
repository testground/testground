package smlbench

import "github.com/ipfs/testground/sdk/runtime"

var (
	MetricTimeToAdd     = &runtime.MetricDefinition{Name: "time_to_add", Unit: "ms", ImprovementDir: -1}
	MetricTimeToConnect = &runtime.MetricDefinition{Name: "time_to_connect", Unit: "ms", ImprovementDir: -1}
	MetricTimeToGet     = &runtime.MetricDefinition{Name: "time_to_get", Unit: "ms", ImprovementDir: -1}
)
