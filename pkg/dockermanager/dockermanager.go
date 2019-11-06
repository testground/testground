package dockermanager

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"

	"github.com/ipfs/testground/pkg/logging"
)

var eventFilter = filters.NewArgs(
	filters.Arg("type", "container"),
	filters.Arg("status", "start"),
	filters.Arg("status", "stop"),
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
func (dm *Manager) Close() error {
	return dm.Client.Close()
}

// Container is a convenient handle for a docker container.
type Container struct {
	logging.Logging
	ID      string
	Manager *Manager
}

// Inspect inspects this docker container.
func (dc *Container) Inspect(ctx context.Context) (types.ContainerJSON, error) {
	return dc.Manager.ContainerInspect(ctx, dc.ID)
}

// IsOnline returns whether or not the container is online.
func (dc *Container) IsOnline(ctx context.Context) (bool, error) {
	info, err := dc.Inspect(ctx)
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

func (dc *Container) Exec(ctx context.Context, cmd ...string) error {
	resp, err := dc.Manager.ContainerExecCreate(ctx, dc.ID, types.ExecConfig{
		User:       "root",
		Privileged: true,
		Cmd:        cmd,
	})
	if err != nil {
		return err
	}
	return dc.Manager.ContainerExecStart(ctx, resp.ID, types.ExecStartCheck{})
}

// Manage runs and stops workers as containers start and stop.
//
// If you pass labels, only containers labeled with at least one of the given
// labels will be managed.
func (dm *Manager) Manage(
	ctx context.Context,
	worker func(context.Context, *Container) error,
	labels ...string,
) error {

	// Construct the filter.
	filters := eventFilter.Clone()
	for _, l := range labels {
		filters.Add("label", l)
	}

	// Manage workers.
	managers := make(map[string]struct {
		done   chan struct{}
		cancel context.CancelFunc
	})

	defer func() {
		// cancel the remaining managers
		for _, m := range managers {
			m.cancel()
		}
		// wait for the running managers to exit
		for _, m := range managers {
			<-m.done
		}
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stop := func(container string) {
		if m, ok := managers[container]; ok {
			m.cancel()
			delete(managers, container)
			<-m.done
		}
	}
	start := func(container string) {
		if _, ok := managers[container]; ok {
			return
		}

		cctx, cancel := context.WithCancel(ctx)
		done := make(chan struct{})
		managers[container] = struct {
			done   chan struct{}
			cancel context.CancelFunc
		}{
			done:   done,
			cancel: cancel,
		}
		go func() {
			defer close(done)
			handle := dm.NewHandle(container)
			err := worker(cctx, handle)
			if err != nil {
				handle.S().Errorf("sidecar worker failed: %s", err)
			}
		}()
	}

	// Manage existing containers.
	now := time.Now()
	nodes, err := dm.Client.ContainerList(ctx, types.ContainerListOptions{
		Quiet:   true,
		Limit:   -1,
		Filters: filters,
	})

	if err != nil {
		return err
	}

	for _, n := range nodes {
		start(n.ID)
	}

	// Manage new containers.
	eventCh, errs := dm.Client.Events(ctx, types.EventsOptions{
		Filters: filters,
		Since:   now.String(), // skip containers we're already managing.
	})

	for {
		select {
		case event := <-eventCh:
			switch event.Status {
			case "start":
				start(event.ID)
			case "stop":
				stop(event.ID)
			default:
				return fmt.Errorf("unexpected event: type=%s, status=%s", event.Type, event.Status)
			}
		case err := <-errs:
			return err
		}
	}
}

// NewHandle constructs a handle for the given container.
func (dm *Manager) NewHandle(container string) *Container {
	return &Container{
		Logging: logging.NewLogging(dm.S().With("container", container).Desugar()),
		ID:      container,
		Manager: dm,
	}
}
