package smlbench

import tpipeline "github.com/ipfs/test-pipeline"

var (
	MetricTimeToAdd     = &tpipeline.MetricDefinition{Name: "time_to_add", Unit: "ms", ImprovementDir: -1}
	MetricTimeToConnect = &tpipeline.MetricDefinition{Name: "time_to_connect", Unit: "ms", ImprovementDir: -1}
	MetricTimeToGet     = &tpipeline.MetricDefinition{Name: "time_to_get", Unit: "ms", ImprovementDir: -1}
)
