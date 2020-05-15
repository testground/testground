package containerd

import (
	"context"
	"errors"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/davecgh/go-spew/spew"
)

type WorkerFn func(context.Context, *ContainerRef) error

type Manager struct {
	Client *containerd.Client
}

type ContainerRef struct {
	ID      string
	Manager *Manager
}

func NewManager() *Manager {
	containerd, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		panic(err)
	}

	return &Manager{
		Client: containerd,
	}
}

func (c *ContainerRef) Id() string {
	return c.ID
}

func (c *ContainerRef) Env(ctx context.Context) ([]string, error) {
	container, err := c.Manager.Client.LoadContainer(ctx, c.ID)
	if err != nil {
		return nil, err
	}

	spew.Dump(container)

	return nil, errors.New("wtf env")
}

func (c *ContainerRef) Hostname(ctx context.Context) (string, error) {
	container, err := c.Manager.Client.LoadContainer(ctx, c.ID)
	if err != nil {
		return "", err
	}

	spew.Dump(container)

	return "", errors.New("wtf hostname")
}

func (c *ContainerRef) Labels(ctx context.Context) (map[string]string, error) {
	container, err := c.Manager.Client.LoadContainer(ctx, c.ID)
	if err != nil {
		return nil, err
	}

	spew.Dump(container.Labels(ctx))

	return nil, errors.New("wtf labels")
}

func (c *ContainerRef) Pid(ctx context.Context) (int, error) {
	container, err := c.Manager.Client.LoadContainer(ctx, c.ID)
	if err != nil {
		return 0, err
	}

	spew.Dump(container.Labels(ctx))

	return 0, errors.New("pid labels")
}

func (c *ContainerRef) IsRunning(ctx context.Context) (bool, error) {
	container, err := c.Manager.Client.LoadContainer(ctx, c.ID)
	if err != nil {
		return false, err
	}

	task, err := container.Task(ctx, cio.Load)
	if err != nil {
		return false, err
	}
	status, err := task.Status(ctx)
	if err != nil {
		return false, err
	}
	if status.Status == containerd.Running {
		return true, nil
	}

	return false, nil
}

func (m *Manager) Watch(ctx context.Context, worker WorkerFn, labels ...string) error {
	return nil
}

func (m *Manager) Close() error {
	return m.Client.Close()
}
