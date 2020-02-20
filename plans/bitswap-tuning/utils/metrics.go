package utils

import (
	"fmt"

	"github.com/ipfs/testground/sdk/runtime"
)

var (
	MetricTimeToFetch = func(id string) *runtime.MetricDefinition {
		return &runtime.MetricDefinition{Name: fmt.Sprintf("%s/time_to_fetch", id), Unit: "ns", ImprovementDir: -1}
	}
	MetricMsgsRcvd = func(id string) *runtime.MetricDefinition {
		return &runtime.MetricDefinition{Name: fmt.Sprintf("%s/msgs_rcvd", id), Unit: "messages", ImprovementDir: -1}
	}
	MetricDataSent = func(id string) *runtime.MetricDefinition {
		return &runtime.MetricDefinition{Name: fmt.Sprintf("%s/data_sent", id), Unit: "bytes", ImprovementDir: -1}
	}
	MetricDataRcvd = func(id string) *runtime.MetricDefinition {
		return &runtime.MetricDefinition{Name: fmt.Sprintf("%s/data_rcvd", id), Unit: "bytes", ImprovementDir: -1}
	}
	MetricDupDataRcvd = func(id string) *runtime.MetricDefinition {
		return &runtime.MetricDefinition{Name: fmt.Sprintf("%s/dup_data_rcvd", id), Unit: "bytes", ImprovementDir: -1}
	}
	MetricBlksSent = func(id string) *runtime.MetricDefinition {
		return &runtime.MetricDefinition{Name: fmt.Sprintf("%s/blks_sent", id), Unit: "blocks", ImprovementDir: -1}
	}
	MetricBlksRcvd = func(id string) *runtime.MetricDefinition {
		return &runtime.MetricDefinition{Name: fmt.Sprintf("%s/blks_rcvd", id), Unit: "blocks", ImprovementDir: -1}
	}
	MetricDupBlksRcvd = func(id string) *runtime.MetricDefinition {
		return &runtime.MetricDefinition{Name: fmt.Sprintf("%s/dup_blks_rcvd", id), Unit: "blocks", ImprovementDir: -1}
	}
)
