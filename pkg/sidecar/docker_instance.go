//+build linux

package sidecar

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/ipfs/testground/pkg/dockermanager"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

const (
	EnvRedisHost = "REDIS_HOST"
)

type link struct {
	netlink.Link
	IPv4, IPv6 *net.IPNet
}

// dockerLinks maps a set of container networks to container link devices.
func dockerLinks(nl *netlink.Handle, nsettings *types.NetworkSettings) (map[string]link, error) {
	type dnet struct {
		id       string
		ip4, ip6 *net.IPNet
	}
	macToNet := make(map[string]dnet, len(nsettings.Networks))
	for _, network := range nsettings.Networks {
		n := dnet{
			id: network.NetworkID,
		}
		if network.IPAddress != "" {
			ip := net.ParseIP(network.IPAddress)
			if ip == nil {
				return nil, fmt.Errorf("failed to parse ipv4 %s addrs", network.IPAddress)
			}
			n.ip4 = &net.IPNet{
				IP:   ip,
				Mask: net.CIDRMask(network.IPPrefixLen, 8*net.IPv4len),
			}
		}
		if network.GlobalIPv6Address != "" {
			ip := net.ParseIP(network.GlobalIPv6Address)
			if ip == nil {
				return nil, fmt.Errorf("failed to parse ipv6 %s addrs", network.GlobalIPv6Address)
			}
			n.ip6 = &net.IPNet{
				IP:   ip,
				Mask: net.CIDRMask(network.GlobalIPv6PrefixLen, 8*net.IPv6len),
			}
		}
		macToNet[network.MacAddress] = n
	}

	links, err := nl.LinkList()
	if err != nil {
		return nil, err
	}
	linkMap := make(map[string]link, len(links))
	for _, l := range links {
		if n, ok := macToNet[l.Attrs().HardwareAddr.String()]; ok {
			linkMap[n.id] = link{
				Link: l,
				IPv4: n.ip4,
				IPv6: n.ip6,
			}
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
	redisHost := os.Getenv(EnvRedisHost)

	redisIp, err := net.ResolveIPAddr("ip4", redisHost)
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
		switch {
		case err != nil:
			return fmt.Errorf("when initializing the container: %w", err)
		case inst == nil:
			// not using the sidecar
			return nil
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

	// Construct the runtime environment
	runenv, err := runtime.ParseRunEnv(info.Config.Env)
	if err != nil {
		return nil, fmt.Errorf("failed to parse run environment: %w", err)
	}

	// Not using the sidecar, ignore this container.
	if !runenv.TestSidecar {
		return nil, nil
	}

	////////////
	//  LOGS  //
	////////////

	logs, err := container.Logs(ctx)
	if err != nil {
		return nil, err
	}

	//////////////////
	//  NETWORKING  //
	//////////////////

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
	redisRoutes, err := netlinkHandle.RouteGet(d.redis)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve route to redis: %w", err)
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
	return NewInstance(runenv, info.Config.Hostname, network, newDockerLogs(logs))
}

type dockerLink struct {
	*NetlinkLink
	IPv4, IPv6 *net.IPNet
}

type DockerNetwork struct {
	container      *dockermanager.Container
	activeLinks    map[string]*dockerLink // name -> link handle
	availableLinks map[string]string      // name -> id
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
	for name := range dn.activeLinks {
		networks = append(networks, name)
	}
	return networks
}

func (dn *DockerNetwork) ConfigureNetwork(ctx context.Context, cfg *sync.NetworkConfig) error {
	netId, available := dn.availableLinks[cfg.Network]
	if !available {
		return fmt.Errorf("unsupported network: %s", cfg.Network)
	}

	link, online := dn.activeLinks[cfg.Network]

	// Are we _disabling_ the network?
	if !cfg.Enable {
		// Yes, is it already disabled?
		if online {
			// No. Disconnect.
			if err := dn.container.Manager.NetworkDisconnect(ctx, netId, dn.container.ID, true); err != nil {
				return err
			}
			delete(dn.activeLinks, cfg.Network)
		}
		return nil
	}

	if online && ((cfg.IPv6 != nil && !link.IPv6.IP.Equal(cfg.IPv6.IP)) ||
		(cfg.IPv4 != nil && !link.IPv4.IP.Equal(cfg.IPv4.IP))) {
		// Disconnect and reconnect to change the IP addresses.
		//
		// NOTE: We probably don't need to do this on local docker.
		// However, we probably do with swarm.
		online = false
		if err := dn.container.Manager.NetworkDisconnect(ctx, netId, dn.container.ID, true); err != nil {
			return err
		}
		delete(dn.activeLinks, cfg.Network)
	}

	// Are we _connected_ to the network.
	if !online {
		// No, we're not.
		// Connect.
		ipamConfig := network.EndpointIPAMConfig{}
		if cfg.IPv4 != nil {
			ipamConfig.IPv4Address = cfg.IPv4.IP.String()
		}
		if cfg.IPv6 != nil {
			ipamConfig.IPv6Address = cfg.IPv6.IP.String()
		}

		if err := dn.container.Manager.NetworkConnect(
			ctx,
			netId,
			dn.container.ID,
			&network.EndpointSettings{
				IPAMConfig: &ipamConfig,
			},
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
		handle, err := NewNetlinkLink(dn.nl, linkInfo.Link)
		if err != nil {
			return err
		}
		link = &dockerLink{
			NetlinkLink: handle,
			IPv4:        linkInfo.IPv4,
			IPv6:        linkInfo.IPv6,
		}
		dn.activeLinks[cfg.Network] = link
	}

	// We don't yet support applying per-subnet rules.
	if len(cfg.Rules) != 0 {
		return fmt.Errorf("TODO: per-subnet bandwidth rules not supported")
	}

	if err := link.Shape(cfg.Default); err != nil {
		return err
	}
	return nil
}
