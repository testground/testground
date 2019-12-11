package utils

import "github.com/ipfs/testground/sdk/runtime"

var (
	MetricTimeToFetch = &runtime.MetricDefinition{Name: "time_to_fetch", Unit: "ns", ImprovementDir: -1}
	MetricMsgsRcvd    = &runtime.MetricDefinition{Name: "msgs_rcvd", Unit: "messages", ImprovementDir: -1}
	MetricDataSent    = &runtime.MetricDefinition{Name: "data_sent", Unit: "bytes", ImprovementDir: -1}
	MetricDataRcvd    = &runtime.MetricDefinition{Name: "data_rcvd", Unit: "bytes", ImprovementDir: -1}
	MetricDupDataRcvd = &runtime.MetricDefinition{Name: "dup_data_rcvd", Unit: "bytes", ImprovementDir: -1}
	MetricBlksSent    = &runtime.MetricDefinition{Name: "blks_sent", Unit: "blocks", ImprovementDir: -1}
	MetricBlksRcvd    = &runtime.MetricDefinition{Name: "blks_rcvd", Unit: "blocks", ImprovementDir: -1}
	MetricDupBlksRcvd = &runtime.MetricDefinition{Name: "dup_blks_rcvd", Unit: "blocks", ImprovementDir: -1}
)
