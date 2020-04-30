package sidecar

import (
	"context"
	"errors"
	"github.com/testground/sdk-go/sync"
	gosync "sync"
)

// Runner
func NewMockReactor() (Reactor, error) {
	return &MockReactor{}, nil
}

// Reactor
type MockReactor struct{}

func (*MockReactor) Close() error { return nil }

func (*MockReactor) Handle(ctx context.Context, handler InstanceHandler) error { return nil }

func NewMockNetwork() *MockNetwork {
	active := make([]string, 0)
	configured := make([]*sync.NetworkConfig, 0)
	mux := gosync.Mutex{}
	return &MockNetwork{
		Active:     active,
		Configured: configured,
		Closed:     false,
		L:          &mux,
	}

}

// Network
type MockNetwork struct {
	Active     []string
	Configured []*sync.NetworkConfig
	Closed     bool
	L          gosync.Locker
}

func (m *MockNetwork) Close() error {
	m.Closed = true
	return nil
}

func (m *MockNetwork) ConfigureNetwork(ctx context.Context, cfg *sync.NetworkConfig) error {
	if m.Closed {
		return errors.New("mock network is closed.")
	}
	m.L.Lock()
	m.Configured = append(m.Configured, cfg)
	m.L.Unlock()
	return nil
}

func (m *MockNetwork) ListActive() []string {
	return m.Active
}
