//go:build linux
// +build linux

package sidecar

import (
	"fmt"
	"net"

	"github.com/docker/docker/api/types"
	"github.com/vishvananda/netlink"
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
		n := dnet{id: network.NetworkID}

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
			linkMap[n.id] = link{Link: l, IPv4: n.ip4, IPv6: n.ip6}
		}
	}

	return linkMap, nil
}
