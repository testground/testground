package utils

import (
	"strconv"

	"github.com/ipfs/testground/sdk/runtime"
)

func MakeTimeToAddMetric(size int64, method string) *runtime.MetricDefinition {
	return &runtime.MetricDefinition{
		Name:           "time_to_add_" + method + "_" + strconv.FormatInt(size, 10),
		Unit:           "ms",
		ImprovementDir: -1,
	}
}
