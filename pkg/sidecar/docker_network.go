//+build linux

package sidecar

import (
	"context"
	"fmt"
	"net"

	"github.com/testground/sdk-go/sync"
	"github.com/testground/testground/pkg/docker"

	"github.com/docker/docker/api/types/network"
	"github.com/vishvananda/netlink"
)

type dockerLink struct {
	*NetlinkLink
	IPv4, IPv6 *net.IPNet
}

type DockerNetwork struct {
	container      *docker.ContainerRef
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
