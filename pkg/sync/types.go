package sync

import "context"

// Service is the implementation of a sync service. This service must support synchronization
// actions such as pub-sub and barriers.
type Service interface {
	Publish(ctx context.Context, topic string, payload interface{}) (seq int64, err error)
	Subscribe(ctx context.Context, topic string) (*Subscription, error)
	Barrier(ctx context.Context, state string, target int64) error
	SignalEntry(ctx context.Context, state string) (after int64, err error)
}

// Subscription represents a subscription to a certain topic to which
// other instances can publish.
type Subscription struct {
	outCh  chan interface{}
	doneCh chan error
}

// PublishRequest represents a publish request.
type PublishRequest struct {
	Topic   string      `json:"topic"`
	Payload interface{} `json:"payload"`
}

// PublishResponse represents a publish response.
type PublishResponse struct {
	Seq int64 `json:"seq"`
}

// SubscribeRequest represents a subscribe request.
type SubscribeRequest struct {
	Topic string `json:"topic"`
}

// BarrierRequest represents a barrier response.
type BarrierRequest struct {
	State  string `json:"state"`
	Target int64  `json:"target"`
}

// SignalEntryRequest represents a signal entry request.
type SignalEntryRequest struct {
	State string `json:"state"`
}

// SignalEntryResponse represents a signal entry response.
type SignalEntryResponse struct {
	Seq int64 `json:"seq"`
}

// Request represents a request from the test instance to the sync service.
// The request ID must be present and one of the requests must be non-nil.
// The ID will be used on further responses.
type Request struct {
	ID                 string              `json:"id"`
	IsCancel           bool                `json:"is_cancel"`
	PublishRequest     *PublishRequest     `json:"publish,omitempty"`
	SubscribeRequest   *SubscribeRequest   `json:"subscribe,omitempty"`
	BarrierRequest     *BarrierRequest     `json:"barrier,omitempty"`
	SignalEntryRequest *SignalEntryRequest `json:"signal_entry,omitempty"`
}

// Response represents a response from the sync service to a test instance.
// The response ID must be present and one of the response types of Error must
// be non-nil. The ID is the same as the request ID.
type Response struct {
	ID                  string               `json:"id"`
	Error               string               `json:"error"`
	PublishResponse     *PublishResponse     `json:"publish"`
	SubscribeResponse   interface{}          `json:"subscribe"`
	SignalEntryResponse *SignalEntryResponse `json:"signal_entry"`
}
