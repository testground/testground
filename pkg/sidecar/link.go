//go:build linux
// +build linux

package sidecar

import (
	"fmt"
	"math"
	"net"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"

	"github.com/testground/sdk-go/network"
)

var (
	rootHandle    = netlink.MakeHandle(1, 0)
	defaultHandle = netlink.MakeHandle(1, 2)
)

// NetlinkLink abstracts operations over a network interface.
//
// NetlinkLink shapes the egress traffic on the link using TC. To do so, it
// configures the following TC tree:
//
//     [________HTB Qdisc_________] - root
//        0 |      1 |     n | ...  - queue; 0 is the default.
//     [HTB Class]                  - bandwidth (rate limiting)
//          |
//     [Netem Qdisc]                - latency, jitter, etc. (per-packet attributes)
//
// At the moment, only one queue is supported. When support for multiple subnets
// is added, additional classes/queues will be added per-subnet.
//
// NetlinkLink also supports setting the network device up/down and changing the
// IP address.
//
// NOTE: Not all run environments will react well to the IP address changing.
// Don't use this feature with docker.
type NetlinkLink struct {
	netlink.Link
	handle *netlink.Handle
}

// NewNetlinkLink constructs a new netlink link handle.
func NewNetlinkLink(handle *netlink.Handle, link netlink.Link) (*NetlinkLink, error) {
	// TODO: multiple networks.
	root := netlink.NewHtb(netlink.QdiscAttrs{
		LinkIndex: link.Attrs().Index,
		Parent:    netlink.HANDLE_ROOT,
		Handle:    rootHandle,
	})
	root.Defcls = defaultHandle

	if err := handle.QdiscAdd(root); err != nil {
		return nil, fmt.Errorf("failed to set root qdisc: %w", err)
	}

	l := &NetlinkLink{Link: link, handle: handle}

	if err := l.init(0); err != nil {
		return nil, err
	}

	return l, nil
}

// Each "class" will have two handles:
//
// * htb: 1:(idx+2)
// * netem: (idx+2):0
func handlesForIndex(idx uint16) (htb, netem uint32) {
	id := idx + 2
	return netlink.MakeHandle(1, id), netlink.MakeHandle(id, 0)
}

// Initialize the class with index `idx`. For now, we only init a single
// (default) class. In the future, we'll create one per network. On _each_
// interface.
//
// We can then specify egress propreties per-network by mapping traffic to each
// of these classes using filters.
func (l *NetlinkLink) init(idx uint16) error {
	htbHandle, netemHandle := handlesForIndex(idx)
	htbAttrs := netlink.ClassAttrs{
		LinkIndex: l.Attrs().Index,
		Parent:    rootHandle,
		Handle:    htbHandle,
	}

	netemAttrs := netlink.QdiscAttrs{
		LinkIndex: l.Attrs().Index,
		Parent:    htbHandle,
		Handle:    netemHandle,
	}

	if err := l.handle.ClassAdd(netlink.NewHtbClass(
		htbAttrs,
		netlink.HtbClassAttrs{
			Rate: math.MaxUint64,
		},
	)); err != nil {
		return fmt.Errorf("failed to initialize htb class: %w", err)
	}

	if err := l.handle.QdiscAdd(netlink.NewNetem(
		netemAttrs,
		netlink.NetemQdiscAttrs{},
	)); err != nil {
		return fmt.Errorf("failed to initialize netem qdisc: %w", err)
	}

	return nil
}

// Sets link's HTB class attributes. See tc-htb(8).
func (l *NetlinkLink) setHtb(idx uint16, attrs netlink.HtbClassAttrs) error {
	htbHandle, _ := handlesForIndex(idx)
	return l.handle.ClassChange(netlink.NewHtbClass(
		netlink.ClassAttrs{
			LinkIndex: l.Attrs().Index,
			Parent:    rootHandle,
			Handle:    htbHandle,
		},
		attrs,
	))
}

// Sets link's Netem queuing disciplines attributes. See tc-netem(8).
func (l *NetlinkLink) setNetem(idx uint16, attrs netlink.NetemQdiscAttrs) error {
	htbHandle, netemHandle := handlesForIndex(idx)
	return l.handle.QdiscChange(netlink.NewNetem(
		netlink.QdiscAttrs{
			LinkIndex: l.Attrs().Index,
			Parent:    htbHandle,
			Handle:    netemHandle,
		},
		attrs,
	))
}

func toMicroseconds(t time.Duration) uint32 {
	us := t.Microseconds()
	if us > math.MaxUint32 {
		// might as well just chop this off.
		// This is ~1 hour
		us = math.MaxUint32
	}
	return uint32(us)
}

// Shape applies the link "shape" to the link, setting the bandwidth, latency,
// jitter, etc.
func (l *NetlinkLink) Shape(shape network.LinkShape) error {
	rate := shape.Bandwidth
	if rate == 0 {
		rate = math.MaxUint64
	}

	// TODO: eventually, we'll have a queue per-subnet to allow shaping per-subnet.

	if err := l.setHtb(0, netlink.HtbClassAttrs{
		Rate: rate,
	}); err != nil {
		return err
	}

	if err := l.setNetem(0, netlink.NetemQdiscAttrs{
		Jitter:        toMicroseconds(shape.Jitter),
		Latency:       toMicroseconds(shape.Latency),
		Loss:          shape.Loss,
		CorruptProb:   shape.Corrupt,
		CorruptCorr:   shape.CorruptCorr,
		ReorderProb:   shape.Reorder,
		ReorderCorr:   shape.ReorderCorr,
		Duplicate:     shape.Duplicate,
		DuplicateCorr: shape.DuplicateCorr,
	}); err != nil {
		return err
	}
	return nil
}

// TODO(cory) actually process the shape per network.
// For now, this simply adds a route based on the Filter
func (l *NetlinkLink) AddRules(rules []network.LinkRule) error {
	for _, rule := range rules {
		dropRoute := nl.FR_ACT_BLACKHOLE
		rejectRoute := nl.FR_ACT_PROHIBIT
		r := netlink.Route{
			Dst: &rule.Subnet.IPNet,
		}
		switch rule.Filter {
		// delete drop and reject routes, if they exist.
		case network.Accept:
			r.Type = dropRoute
			_ = l.handle.RouteDel(&r)
			r.Type = rejectRoute
			_ = l.handle.RouteDel(&r)
			continue

		// Setup a reject route.
		case network.Reject:
			r.Type = rejectRoute

		// setup a blackhole route.
		case network.Drop:
			r.Type = dropRoute
		}
		err := l.handle.RouteReplace(&r)
		if err != nil {
			return err
		}
	}
	return nil
}

// NOTE: None of the following methods are currently used. They exist for future
// non-docker runners.

// AddrAdd adds an address to the link.
//
// NOTE: This won't work in docker; use docker connect/disconnect.
func (l *NetlinkLink) AddrAdd(ip *net.IPNet) error {
	return l.handle.AddrAdd(l.Link, &netlink.Addr{IPNet: ip})
}

// AddrDel removes an address from the link.
//
// NOTE: This won't work in docker; use docker connect/disconnect.
func (l *NetlinkLink) AddrDel(ip *net.IPNet) error {
	return l.handle.AddrDel(l.Link, &netlink.Addr{IPNet: ip})
}

// ListV4 lists all IPv4 addresses associated with the link.
func (l *NetlinkLink) ListV4() ([]*net.IPNet, error) {
	return l.list(netlink.FAMILY_V4)
}

// ListV6 lists all IPv6 addresses associated with the link.
func (l *NetlinkLink) ListV6() ([]*net.IPNet, error) {
	return l.list(netlink.FAMILY_V6)
}

func (l *NetlinkLink) list(family int) ([]*net.IPNet, error) {
	addrs, err := l.handle.AddrList(l.Link, family)
	if err != nil {
		return nil, err
	}
	res := make([]*net.IPNet, len(addrs))
	for i, addr := range addrs {
		res[i] = addr.IPNet
	}
	return res, nil
}

// Up sets the link up.
func (l *NetlinkLink) Up() error {
	return l.handle.LinkSetUp(l.Link)
}

// Down takes the link down.
func (l *NetlinkLink) Down() error {
	return l.handle.LinkSetDown(l.Link)
}
