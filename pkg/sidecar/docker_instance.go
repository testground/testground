package sidecar

import (
	"context"
	"fmt"
	"net"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/ipfs/testground/pkg/dockermanager"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

// dockerLinks maps a set of container networks to container link devices.
func dockerLinks(nl *netlink.Handle, nsettings *types.NetworkSettings) (map[string]netlink.Link, error) {
	macToName := make(map[string]string, len(nsettings.Networks))
	for _, network := range nsettings.Networks {
		macToName[network.MacAddress] = network.NetworkID
	}

	links, err := nl.LinkList()
	if err != nil {
		return nil, err
	}
	linkMap := make(map[string]netlink.Link, len(links))
	for _, link := range links {
		if id, ok := macToName[link.Attrs().HardwareAddr.String()]; ok {
			linkMap[id] = link
		}
	}
	return linkMap, nil
}

type DockerInstanceManager struct {
	redis   net.IP
	manager *dockermanager.Manager
}

func NewDockerManager() (InstanceManager, error) {
	// TODO: Generalize this to a list of services.
	redisIp, err := net.ResolveIPAddr("ip4", "testground-redis")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve redis host: %w", err)
	}

	docker, err := dockermanager.NewManager()
	if err != nil {
		return nil, err
	}

	return &DockerInstanceManager{
		manager: docker,
		redis:   redisIp.IP,
	}, nil
}

func (d *DockerInstanceManager) Manage(
	ctx context.Context,
	worker func(ctx context.Context, inst *Instance) error,
) error {
	return d.manager.Manage(ctx, func(ctx context.Context, container *dockermanager.Container) error {
		inst, err := d.manageContainer(ctx, container)
		if err != nil {
			return fmt.Errorf("when initializing the container: %w", err)
		}
		err = worker(ctx, inst)
		if err != nil {
			return fmt.Errorf("container worker failed: %w", err)
		}
		return nil
	}, "testground.runid")
}

func (d *DockerInstanceManager) Close() error {
	return d.manager.Close()
}

func (d *DockerInstanceManager) manageContainer(ctx context.Context, container *dockermanager.Container) (inst *Instance, err error) {
	// Get the state/config of the cluster
	info, err := container.Inspect(ctx)
	if err != nil {
		return nil, fmt.Errorf("inspect failed: %w", err)
	}

	if !info.State.Running {
		return nil, fmt.Errorf("not running")
	}

	// TODO: cache this?
	networks, err := container.Manager.NetworkList(ctx, types.NetworkListOptions{
		Filters: filters.NewArgs(
			filters.Arg(
				"label",
				"testground.runid="+info.Config.Labels["testground.runid"],
			),
		),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	// Construct the runtime environment

	runenv, err := runtime.ParseRunEnv(info.Config.Env)
	if err != nil {
		return nil, fmt.Errorf("failed to parse run environment: %w", err)
	}

	// Get a netlink handle.
	nshandle, err := netns.GetFromPid(info.State.Pid)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup the net namespace: %s", err)
	}
	defer nshandle.Close()

	netlinkHandle, err := netlink.NewHandleAt(nshandle)
	// TODO: is this safe?
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
		activeLinks:    make(map[string]*NetlinkLink, len(info.NetworkSettings.Networks)),
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
	redisRoutes, err := netlinkHandle.RouteGet(d.redis)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve route to redis: %w", err)
	}

	for id, link := range links {
		if name, ok := reverseIndex[id]; ok {
			// manage this network
			network.activeLinks[name], err = NewNetlinkLink(netlinkHandle, link)
			if err != nil {
				return nil, fmt.Errorf(
					"failed to initialize link %s (%s): %w",
					name,
					link.Attrs().Name,
					err,
				)
			}
			continue
		}

		// We've found a control network (or some other network).

		// Get the current routes.
		linkRoutes, err := netlinkHandle.RouteList(link, netlink.FAMILY_ALL)
		if err != nil {
			return nil, fmt.Errorf("failed to list routes for link %s", link.Attrs().Name)
		}

		// Add specific routes to redis if redis uses this link.
		for _, route := range redisRoutes {
			if route.LinkIndex != link.Attrs().Index {
				continue
			}
			if err := netlinkHandle.RouteAdd(&route); err != nil {
				return nil, fmt.Errorf("failed to add new route: %w", err)
			}
			break
		}

		// Remove the original routes
		for _, route := range linkRoutes {
			if err := netlinkHandle.RouteDel(&route); err != nil {
				return nil, fmt.Errorf("failed to remove existing route: %w", err)
			}
		}
	}
	return NewInstance(runenv, info.Config.Hostname, network)
}

type DockerNetwork struct {
	container      *dockermanager.Container
	activeLinks    map[string]*NetlinkLink // name -> link handle
	availableLinks map[string]string       // name -> id
	nl             *netlink.Handle
}

func (dn *DockerNetwork) Close() error {
	dn.nl.Delete()
	return nil
}

func (dn *DockerNetwork) ListAvailable() []string {
	networks := make([]string, 0, len(dn.availableLinks))
	for network := range dn.availableLinks {
		networks = append(networks, network)
	}
	return networks
}

func (dn *DockerNetwork) ListActive() []string {
	networks := make([]string, 0, len(dn.activeLinks))
	for name := range dn.availableLinks {
		if _, ok := dn.activeLinks[name]; ok {
			networks = append(networks, name)
		}
	}
	return networks
}

func (dn *DockerNetwork) ConfigureNetwork(ctx context.Context, cfg *sync.NetworkConfig) error {
	netId, ok := dn.availableLinks[cfg.Network]
	if !ok {
		return fmt.Errorf("unsupported network: %s", cfg.Network)
	}

	link, ok := dn.activeLinks[cfg.Network]

	// Are we _disabling_ the network?
	if !cfg.Enable {
		// Yes, is it already disabled?
		if ok {
			// No. Disconnect.
			if err := dn.container.Manager.NetworkDisconnect(ctx, netId, dn.container.ID, true); err != nil {
				return err
			}
			delete(dn.activeLinks, netId)
		}
		return nil
	}

	// We don't yet support setting IP addresses.
	if cfg.IP != nil {
		return fmt.Errorf("TODO: IP addresses cannot currently be configured")
	}

	// Are we _connected_ to the network.
	if !ok {
		// No, we're not yet connected.
		// Connect.
		if err := dn.container.Manager.NetworkConnect(
			ctx,
			netId,
			dn.container.ID,
			&network.EndpointSettings{},
		); err != nil {
			return err
		}
		info, err := dn.container.Inspect(ctx)
		if err != nil {
			return err
		}
		// Resolve networks to internal links
		links, err := dockerLinks(dn.nl, info.NetworkSettings)
		if err != nil {
			return err
		}
		// Lookup the new network
		linkInfo, ok := links[netId]
		if !ok {
			return fmt.Errorf("couldn't find network interface for: %s", cfg.Network)
		}
		// Register an active link.
		link, err = NewNetlinkLink(dn.nl, linkInfo)
		if err != nil {
			return err
		}
		dn.activeLinks[netId] = link
	}

	// We don't yet support applying per-subnet rules.
	if len(cfg.Rules) != 0 {
		return fmt.Errorf("TODO: per-subnet bandwidth rules not supported")
	}

	// TODO: is 0 rate correct? How do I set "don't limit". Do I need to delete the rule?
	if err := link.SetBandwidth(cfg.Default.Bandwidth); err != nil {
		return err
	}
	if err := link.SetLatency(cfg.Default.Latency); err != nil {
		return err
	}
	return nil
}
