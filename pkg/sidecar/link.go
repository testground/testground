//+build linux

package sidecar

import (
	"fmt"
	"math"
	"net"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/ipfs/testground/sdk/sync"
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
// * bandwidth: 1:(idx+2)
// * latency: (idx+2):0
func handlesForIndex(idx uint16) (bandwidth, latency uint32) {
	id := idx + 2
	return netlink.MakeHandle(1, id), netlink.MakeHandle(id, 0)
}

// Initialize the class with index `idx`. For now, we only init a single
// (default) class. In the future, we'll create one per network. On _each_
// interface.
//
// We can then specify egress latency/bandwidth per-network by mapping traffic
// to each of these classes using filters.
func (l *NetlinkLink) init(idx uint16) error {
	bandwidthHandle, latencyHandle := handlesForIndex(idx)
	bandwidthAttrs := netlink.ClassAttrs{
		LinkIndex: l.Attrs().Index,
		Parent:    rootHandle,
		Handle:    bandwidthHandle,
	}

	latencyAttrs := netlink.QdiscAttrs{
		LinkIndex: l.Attrs().Index,
		Parent:    bandwidthHandle,
		Handle:    latencyHandle,
	}

	if err := l.handle.ClassAdd(netlink.NewHtbClass(
		bandwidthAttrs,
		netlink.HtbClassAttrs{
			Rate: math.MaxUint64,
		},
	)); err != nil {
		return fmt.Errorf("failed to initialize bandwidth class: %w", err)
	}

	if err := l.handle.QdiscAdd(netlink.NewNetem(
		latencyAttrs,
		netlink.NetemQdiscAttrs{},
	)); err != nil {
		return fmt.Errorf("failed to initialize latency qdisc: %w", err)
	}

	return nil
}

// Sets link's HTB class attributes. See tc-htb(8).
func (l *NetlinkLink) setHtb(idx uint16, attrs netlink.HtbClassAttrs) error {
	bandwidthHandle, _ := handlesForIndex(idx)
	return l.handle.ClassChange(netlink.NewHtbClass(
		netlink.ClassAttrs{
			LinkIndex: l.Attrs().Index,
			Parent:    rootHandle,
			Handle:    bandwidthHandle,
		},
		attrs,
	))
}

// Sets link's Netem queuing disciplines attributes. See tc-netem(8).
func (l *NetlinkLink) setNetem(idx uint16, attrs netlink.NetemQdiscAttrs) error {
	bandwidthHandle, latencyHandle := handlesForIndex(idx)
	return l.handle.QdiscChange(netlink.NewNetem(
		netlink.QdiscAttrs{
			LinkIndex: l.Attrs().Index,
			Parent:    bandwidthHandle,
			Handle:    latencyHandle,
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
func (l *NetlinkLink) Shape(shape sync.LinkShape) error {
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

// Set the links address.
// NOTE: This won't work in docker; use docker connect/disconnect.
func (l *NetlinkLink) SetAddr(ip *net.IPNet) error {
	return l.handle.AddrReplace(l.Link, &netlink.Addr{IPNet: ip})
}

// Up sets the link up.
func (l *NetlinkLink) Up() error {
	return l.handle.LinkSetUp(l.Link)
}

// Down takes the link down.
func (l *NetlinkLink) Down() error {
	return l.handle.LinkSetDown(l.Link)
}
