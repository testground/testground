package testground

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type MetricDefinition struct {
	Name           string `json:"name"`
	Unit           string `json:"unit"`
	ImprovementDir int    `json:"improve_dir"`
}

type Metric struct {
	*MetricDefinition

	Value float64 `json:"value"`
}

type Event struct {
	Context   *TestContext `json:"context"`
	Timestamp int64        `json:"timestamp"`
	Metric    *Metric      `json:"metric"`
}

func EmitMetric(ctx context.Context, def *MetricDefinition, value float64) {
	tctx := ExtractTestContext(ctx)

	evt := &Event{
		Context:   tctx,
		Timestamp: time.Now().UnixNano(),
		Metric:    &Metric{def, value},
	}

	bytes, err := json.Marshal(evt)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(bytes))
}
