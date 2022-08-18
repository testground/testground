package main

import (
	"context"
	"reflect"
	"testing"

	"github.com/testground/sdk-go/sync"
)

type mockSyncClient struct {
	ch chan interface{}
}

func (c *mockSyncClient) Subscribe(ctx context.Context, topic *sync.Topic, ch interface{}) (sub *sync.Subscription, err error) {
	go func() {
		for {
			actualCh := reflect.ValueOf(ch)

			v, ok := <-c.ch
			if !ok {
				break
			}
			actualCh.Send(reflect.ValueOf(v))
		}
	}()
	return nil, nil
}
func (c *mockSyncClient) Publish(ctx context.Context, topic *sync.Topic, payload interface{}) (int64, error) {
	c.ch <- payload
	return 0, nil
}

func (c *mockSyncClient) Close() {
	close(c.ch)
}

func TestLibp2pPingCase(t *testing.T) {
	c1 := &mockSyncClient{ch: make(chan interface{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// node 1
	go libp2pPing(ctx, c1, 1)

	err := libp2pPing(ctx, c1, 2)
	if err != nil {
		t.Fatalf("Failed to ping")
	}

}
