package sync

// Request Types

type PublishRequest struct {
	Topic   string      `json:"topic"`
	Payload interface{} `json:"payload"`
}

type PublishResponse struct {
	Seq int64 `json:"seq"`
}

type SubscribeRequest struct {
	Topic string `json:"topic"`
}

type BarrierRequest struct {
	State  string `json:"state"`
	Target int64  `json:"target"`
}

type SignalEntryRequest struct {
	State string `json:"state"`
}

type SignalEntryResponse struct {
	Seq int64 `json:"seq"`
}

type SignalEventRequest struct {
	Key   string      `json:""`
	Event interface{} `json:"event"`
}

