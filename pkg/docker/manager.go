package docker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/testground/testground/pkg/logging"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

type WorkerFn func(context.Context, *ContainerRef) error

const (
	workerShutdownTimeout = 1 * time.Minute
	workerShutdownTick    = 5 * time.Second
)

// Manager is a convenient wrapper around the docker client.
type Manager struct {
	logging.Logging

	*client.Client
}

// NewManager connects to the local docker instance and provides a convenient
// handle for managing containers.
func NewManager() (*Manager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	return &Manager{
		Logging: logging.NewLogging(logging.S().With("host", cli.DaemonHost()).Desugar()),
		Client:  cli,
	}, nil
}

// Close closes the docker client.
func (m *Manager) Close() error {
	return m.Client.Close()
}

// Container is a convenient handle for a docker container.
type ContainerRef struct {
	logging.Logging

	ID      string
	Manager *Manager
}

// NewContainerRef constructs a reference to a given container.
func (m *Manager) NewContainerRef(id string) *ContainerRef {
	return &ContainerRef{
		Logging: logging.NewLogging(m.S().With("container", id).Desugar()),
		ID:      id,
		Manager: m,
	}
}

// Inspect inspects this docker container.
func (c *ContainerRef) Inspect(ctx context.Context) (types.ContainerJSON, error) {
	return c.Manager.ContainerInspect(ctx, c.ID)
}

// IsOnline returns whether or not the container is online.
func (c *ContainerRef) IsOnline(ctx context.Context) (bool, error) {
	info, err := c.Inspect(ctx)
	if err != nil {
		if client.IsErrNotFound(err) {
			err = nil
		}
		return false, err
	}
	switch info.ContainerJSONBase.State.Status {
	case "running", "paused":
		return true, nil
	}
	return false, nil
}

func (c *ContainerRef) Exec(ctx context.Context, cmd ...string) error {
	resp, err := c.Manager.ContainerExecCreate(ctx, c.ID, types.ExecConfig{
		User:       "root",
		Privileged: true,
		Cmd:        cmd,
	})
	if err != nil {
		return err
	}
	return c.Manager.ContainerExecStart(ctx, resp.ID, types.ExecStartCheck{})
}

// Watch monitors container status, and runs the provider worker for each
// container that starts.
//
// If you pass labels, only containers labeled with at least one of the given
// labels will be managed.
func (m *Manager) Watch(ctx context.Context, worker WorkerFn, labels ...string) error {
	type workerHandle struct {
		done   chan struct{}
		cancel context.CancelFunc
	}

	// Manage workers.
	managers := make(map[string]workerHandle)

	defer func() {
		// wait for the running managers to exit
		// They'll get canceled when we close the main context (deferred
		// below).
		for _, mg := range managers {
			<-mg.done
		}
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stop := func(containerID string) {
		wh, ok := managers[containerID]
		if !ok {
			return
		}

		wh.cancel()
		delete(managers, containerID)

		timeout := time.NewTimer(workerShutdownTimeout)
		defer timeout.Stop()

		ticker := time.NewTicker(workerShutdownTick)
		defer ticker.Stop()

		for {
			select {
			case <-wh.done:
				return
			case <-timeout.C:
				m.S().Panicw("timed out waiting for container worker to stop", "container", containerID)
				return
			case <-ticker.C:
				m.S().Errorw("waiting for container worker to stop", "container", containerID)
			}
		}
	}

	start := func(containerID string) {
		if _, ok := managers[containerID]; ok {
			return
		}

		cctx, cancel := context.WithCancel(ctx)
		done := make(chan struct{})
		managers[containerID] = workerHandle{
			done:   done,
			cancel: cancel,
		}

		go func() {
			defer close(done)
			handle := m.NewContainerRef(containerID)
			err := worker(cctx, handle)
			if err != nil {
				if errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "context canceled") { // docker doesn't wrap errors
					handle.S().Debugf("sidecar worker failed: %s", err)
				} else {
					handle.S().Errorf("sidecar worker failed: %s", err)
				}
			}
		}()
	}

	// Manage existing containers.
	now := time.Now()

	listFilter := filters.NewArgs()
	for _, l := range labels {
		listFilter.Add("label", l)
	}
	nodes, err := m.Client.ContainerList(ctx, types.ContainerListOptions{
		Quiet:   true,
		Limit:   -1,
		Filters: listFilter,
	})

	if err != nil {
		return err
	}

	for _, n := range nodes {
		start(n.ID)
	}

	eventFilter := listFilter.Clone()
	eventFilter.Add("type", "container")
	eventFilter.Add("event", "start")
	eventFilter.Add("event", "stop")
	eventFilter.Add("event", "destroy")
	eventFilter.Add("event", "die")

	// Manage new containers.
	eventCh, errs := m.Client.Events(ctx, types.EventsOptions{
		Filters: eventFilter,
		Since:   now.Format(time.RFC3339Nano),
	})

	for {
		select {
		case event := <-eventCh:
			switch event.Status {
			case "start":
				start(event.ID)
			case "stop", "destroy", "die":
				stop(event.ID)
			default:
				return fmt.Errorf("unexpected event: type=%s, status=%s", event.Type, event.Status)
			}
		case err := <-errs:
			return err
		}
	}
}
