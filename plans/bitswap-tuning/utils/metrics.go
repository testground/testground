package utils

import (
	"fmt"

	"github.com/testground/sdk-go/runtime"
)

var (
	MetricTimeToFetch = func(id string) *runtime.MetricDefinition {
		return &runtime.MetricDefinition{Name: fmt.Sprintf("%s/name:time_to_fetch", id), Unit: "ns", ImprovementDir: -1}
	}
	MetricMsgsRcvd = func(id string) *runtime.MetricDefinition {
		return &runtime.MetricDefinition{Name: fmt.Sprintf("%s/name:msgs_rcvd", id), Unit: "messages", ImprovementDir: -1}
	}
	MetricDataSent = func(id string) *runtime.MetricDefinition {
		return &runtime.MetricDefinition{Name: fmt.Sprintf("%s/name:data_sent", id), Unit: "bytes", ImprovementDir: -1}
	}
	MetricDataRcvd = func(id string) *runtime.MetricDefinition {
		return &runtime.MetricDefinition{Name: fmt.Sprintf("%s/name:data_rcvd", id), Unit: "bytes", ImprovementDir: -1}
	}
	MetricDupDataRcvd = func(id string) *runtime.MetricDefinition {
		return &runtime.MetricDefinition{Name: fmt.Sprintf("%s/name:dup_data_rcvd", id), Unit: "bytes", ImprovementDir: -1}
	}
	MetricBlksSent = func(id string) *runtime.MetricDefinition {
		return &runtime.MetricDefinition{Name: fmt.Sprintf("%s/name:blks_sent", id), Unit: "blocks", ImprovementDir: -1}
	}
	MetricBlksRcvd = func(id string) *runtime.MetricDefinition {
		return &runtime.MetricDefinition{Name: fmt.Sprintf("%s/name:blks_rcvd", id), Unit: "blocks", ImprovementDir: -1}
	}
	MetricDupBlksRcvd = func(id string) *runtime.MetricDefinition {
		return &runtime.MetricDefinition{Name: fmt.Sprintf("%s/name:dup_blks_rcvd", id), Unit: "blocks", ImprovementDir: -1}
	}
)
