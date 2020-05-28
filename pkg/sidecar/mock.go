package sidecar

import (
	"context"
	"errors"
	"math/rand"
	"os"
	"strconv"
	gosync "sync"
	"time"

	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// A MockReactor with the information for a single instance.
// A typicial reactor watches the docker socket for containers. This one creates enough information
// for a single instace, or a single reaction for testing purposes. A mock network and inmem SDK
// client are instantiated here as well.
// To use, instantiate the NewMockReactor and run Handle(). Any messages passed through the inmem
// SDK client are handled as though the come from the mocked instance.
func NewMockReactor() (Reactor, error) {
	unique := strconv.Itoa(rand.Int())
	params := runtime.RunParams{
		TestCase:               "TestCase" + unique,
		TestGroupID:            "TestGroupID" + unique,
		TestGroupInstanceCount: 1,
		TestInstanceCount:      1,
		TestInstanceRole:       "TestInstanceRole" + unique,
		TestPlan:               "TestPlan" + unique,
		TestRun:                unique,
		TestSidecar:            true,
	}
	runenv := runtime.NewRunEnv(params)
	network := NewMockNetwork()
	client := sync.NewInmemClient()
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	return &MockReactor{
		RunParams: &params,
		RunEnv:    runenv,
		Network:   network,
		Client:    client,
		Hostname:  hostname,
	}, nil
}

// Reactor
type MockReactor struct {
	RunParams *runtime.RunParams
	RunEnv    *runtime.RunEnv
	Network   *MockNetwork
	Client    sync.Interface
	Hostname  string
}

func (*MockReactor) Close() error { return nil }

func (r *MockReactor) Handle(ctx context.Context, handler InstanceHandler) error {
	inst, err := NewInstance(r.Client, r.RunEnv, r.Hostname, r.Network)
	if err != nil {
		return err
	}
	return handler(ctx, inst)
}

func NewMockNetwork() *MockNetwork {
	active := map[string]*network.Config{"default": {}}
	configured := make([]*network.Config, 0)
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
	Active     map[string]*network.Config // A map of *active* networks.
	Configured []*network.Config          // A list of all the configurations we've seen
	Closed     bool
	L          gosync.Locker
}

func (m *MockNetwork) Close() error {
	m.Closed = true
	return nil
}

func (m *MockNetwork) ConfigureNetwork(ctx context.Context, cfg *network.Config) error {
	if m.Closed {
		return errors.New("mock network is closed.")
	}
	m.L.Lock()
	defer m.L.Unlock()
	m.Configured = append(m.Configured, cfg)
	m.Active[cfg.Network] = cfg
	return nil
}

func (m *MockNetwork) ListActive() []string {
	var active []string
	for k := range m.Active {
		active = append(active, k)
	}
	return active
}
