package sidecar

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/ipfs/testground/pkg/dockermanager"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

func NewDockerManager() (InstanceManager, error) {
	docker, err := dockermanager.NewManager()
	if err != nil {
		return nil, err
	}
	return (*DockerInstanceManager)(docker), nil
}

type DockerInstanceManager dockermanager.Manager

func (d *DockerInstanceManager) Manage(
	ctx context.Context,
	worker func(ctx context.Context, inst *Instance) error,
) error {
	return (*dockermanager.Manager)(d).Manage(ctx, func(ctx context.Context, container *dockermanager.Container) error {
		inst, err := InstanceFromContainer(ctx, container)
		if err != nil {
			return err
		}
		return worker(ctx, inst)
	}, "testground.runid")
}

func (d *DockerInstanceManager) Close() error {
	return (*dockermanager.Manager)(d).Close()
}

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

// GetInstance gets a handle to the instance associated with the given
// container.
func InstanceFromContainer(ctx context.Context, c *dockermanager.Container) (*Instance, error) {
	// Get the state/config of the cluster
	info, err := c.Inspect(ctx)
	if err != nil {
		return nil, err
	}

	if !info.State.Running {
		return nil, fmt.Errorf("container not running: %s", c.ID)
	}

	// TODO: cache this?
	networks, err := c.Manager.NetworkList(ctx, types.NetworkListOptions{
		Filters: filters.NewArgs(
			filters.Arg(
				"label",
				"testground.runid="+info.Config.Labels["testground.runid"],
			),
		),
	})
	if err != nil {
		return nil, err
	}

	// Construct the runtime environment

	runenv, err := runtime.ParseRunEnv(info.Config.Env)
	if err != nil {
		return nil, err
	}

	// Get a netlink handle.
	nshandle, err := netns.GetFromPath(info.NetworkSettings.SandboxKey)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup the net namespace of %s: %s", c.ID, err)
	}

	netlinkHandle, err := netlink.NewHandleAt(nshandle)
	// TODO: is this safe?
	nshandle.Close() // always close this once we have a netlink handle.

	if err != nil {
		return nil, err
	}

	// Map _current_ networks to links.
	links, err := dockerLinks(netlinkHandle, info.NetworkSettings)
	if err != nil {
		netlinkHandle.Delete()
		return nil, err
	}

	// Finally, construct the network manager.
	network := &DockerNetwork{
		container:      c,
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

	for id, link := range links {
		name, ok := reverseIndex[id]
		if !ok {
			// not a network we want to manage.
			continue
		}
		network.activeLinks[name], err = NewNetlinkLink(netlinkHandle, link)
		if err != nil {
			netlinkHandle.Delete()
			return nil, err
		}
	}
	inst, err := NewInstance(runenv, c.ID, network)
	if err != nil {
		netlinkHandle.Delete()
		return nil, err
	}
	return inst, nil
}

type DockerNetwork struct {
	container      *dockermanager.Container
	activeLinks    map[string]*NetlinkLink // id -> link handle
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
	for name, id := range dn.availableLinks {
		if _, ok := dn.activeLinks[id]; ok {
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

	link, ok := dn.activeLinks[netId]

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
