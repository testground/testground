package smlbench

import "github.com/ipfs/testground/api"

var (
	MetricTimeToAdd     = &api.MetricDefinition{Name: "time_to_add", Unit: "ms", ImprovementDir: -1}
	MetricTimeToConnect = &api.MetricDefinition{Name: "time_to_connect", Unit: "ms", ImprovementDir: -1}
	MetricTimeToGet     = &api.MetricDefinition{Name: "time_to_get", Unit: "ms", ImprovementDir: -1}
)
