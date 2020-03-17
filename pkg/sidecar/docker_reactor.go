//+build linux

package sidecar

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/ipfs/testground/pkg/docker"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/sdk/runtime"
)

type DockerReactor struct {
	routes  []net.IP
	manager *docker.Manager
}

func NewDockerReactor() (Reactor, error) {
	// TODO: Generalize this to a list of services.
	// TODO(cory): prometheus-pushgateway could be added as an env variable as well.
	wantedRoutes := []string{
		os.Getenv(EnvRedisHost),
		"prometheus-pushgateway",
	}

	var resolvedRoutes []net.IP
	for _, route := range wantedRoutes {
		ip, err := net.ResolveIPAddr("ip4", route)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve host %s: %w", route, err)
		}
		resolvedRoutes = append(resolvedRoutes, ip.IP)
	}

	docker, err := docker.NewManager()
	if err != nil {
		return nil, err
	}

	return &DockerReactor{
		routes:  resolvedRoutes,
		manager: docker,
	}, nil
}

func (d *DockerReactor) Handle(globalctx context.Context, handler InstanceHandler) error {
	return d.manager.Watch(globalctx, func(ctx context.Context, container *docker.ContainerRef) error {
		logging.S().Debugw("got container", "container", container.ID)
		inst, err := d.handleContainer(ctx, container)
		if err != nil {
			return fmt.Errorf("failed to initialise the container: %w", err)
		}
		if inst == nil {
			logging.S().Debugw("ignoring container", "container", container.ID)
			return nil
		}

		err = handler(ctx, inst)
		if err != nil {
			return fmt.Errorf("container worker failed: %w", err)
		}
		return nil
	}, "testground.run_id")
}

func (d *DockerReactor) Close() error {
	return d.manager.Close()
}

func (d *DockerReactor) handleContainer(ctx context.Context, container *docker.ContainerRef) (inst *Instance, err error) {
	// Get the state/config of the cluster
	info, err := container.Inspect(ctx)
	if err != nil {
		return nil, fmt.Errorf("inspect failed: %w", err)
	}

	if !info.State.Running {
		return nil, fmt.Errorf("not running")
	}

	// Construct the runtime environment
	params, err := runtime.ParseRunParams(info.Config.Env)
	if err != nil {
		return nil, fmt.Errorf("failed to parse run environment: %w", err)
	}

	// Not using the sidecar, ignore this container.
	if !params.TestSidecar {
		return nil, nil
	}

	// Remove the TestOutputsPath. We can't store anything from the sidecar.
	params.TestOutputsPath = ""
	runenv := runtime.NewRunEnv(*params)

	//////////////////
	//  NETWORKING  //
	//////////////////

	// TODO: cache this?
	networks, err := container.Manager.NetworkList(ctx, types.NetworkListOptions{
		Filters: filters.NewArgs(
			filters.Arg(
				"label",
				"testground.run_id="+info.Config.Labels["testground.run_id"],
			),
		),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	// Get a netlink handle.
	nshandle, err := netns.GetFromPid(info.State.Pid)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup the net namespace: %s", err)
	}
	defer nshandle.Close()

	netlinkHandle, err := netlink.NewHandleAt(nshandle)
	if err != nil {
		return nil, fmt.Errorf("failed to get handle to network namespace: %w", err)
	}

	defer func() {
		if err != nil {
			netlinkHandle.Delete()
		}
	}()

	// Map _current_ networks to links.
	links, err := dockerLinks(netlinkHandle, info.NetworkSettings)
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate links: %w", err)
	}

	// Finally, construct the network manager.
	network := &DockerNetwork{
		container:      container,
		activeLinks:    make(map[string]*dockerLink, len(info.NetworkSettings.Networks)),
		availableLinks: make(map[string]string, len(networks)),
		nl:             netlinkHandle,
	}

	for _, n := range networks {
		name := n.Labels["testground.name"]
		id := n.ID
		network.availableLinks[name] = id
	}

	reverseIndex := make(map[string]string, len(network.availableLinks))
	for name, id := range network.availableLinks {
		reverseIndex[id] = name
	}

	// TODO: Some of this code could be factored out into helpers.

	// Get the routes to redis. We need to keep these.

	var controlRoutes []netlink.Route
	for _, route := range d.routes {
		nlroutes, err := netlinkHandle.RouteGet(route)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve route: %w", err)
		}
		controlRoutes = append(controlRoutes, nlroutes...)
	}

	for id, link := range links {
		if name, ok := reverseIndex[id]; ok {
			// manage this network
			handle, err := NewNetlinkLink(netlinkHandle, link.Link)
			if err != nil {
				return nil, fmt.Errorf(
					"failed to initialize link %s (%s): %w",
					name,
					link.Attrs().Name,
					err,
				)
			}
			network.activeLinks[name] = &dockerLink{
				NetlinkLink: handle,
				IPv4:        link.IPv4,
				IPv6:        link.IPv6,
			}
			continue
		}

		// We've found a control network (or some other network).

		// Get the current routes.
		linkRoutes, err := netlinkHandle.RouteList(link, netlink.FAMILY_ALL)
		if err != nil {
			return nil, fmt.Errorf("failed to list routes for link %s", link.Attrs().Name)
		}

		// Add learned routes plan containers so they can reach  the testground infra on the control network.
		for _, route := range controlRoutes {
			if route.LinkIndex != link.Attrs().Index {
				continue
			}
			if err := netlinkHandle.RouteAdd(&route); err != nil {
				return nil, fmt.Errorf("failed to add new route: %w", err)
			}
		}

		// Remove the original routes
		for _, route := range linkRoutes {
			if err := netlinkHandle.RouteDel(&route); err != nil {
				return nil, fmt.Errorf("failed to remove existing route: %w", err)
			}
		}
	}
	return NewInstance(ctx, runenv, info.Config.Hostname, network)
}
