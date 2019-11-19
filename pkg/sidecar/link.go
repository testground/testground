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

// Link abstracts operations over a network interface.
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

// Sets the bandwidth (in bytes per second).
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

// Sets the latency.
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

func toUs(t time.Duration) uint32 {
	us := t.Microseconds()
	if us > math.MaxUint32 {
		// might as well just chop this off.
		// This is ~1 hour
		us = math.MaxUint32
	}
	return uint32(us)
}

func (l *NetlinkLink) Shape(shape sync.LinkShape) error {
	rate := shape.Bandwidth
	if rate == 0 {
		rate = math.MaxUint64
	}

	if err := l.setHtb(0, netlink.HtbClassAttrs{
		Rate: rate,
	}); err != nil {
		return err
	}

	if err := l.setNetem(0, netlink.NetemQdiscAttrs{
		Jitter:        toUs(shape.Jitter),
		Latency:       toUs(shape.Latency),
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

func (l *NetlinkLink) SetAddr(ip *net.IPNet) error {
	return l.handle.AddrReplace(l.Link, &netlink.Addr{IPNet: ip})
}

func (l *NetlinkLink) Up() error {
	return l.handle.LinkSetUp(l.Link)
}

func (l *NetlinkLink) Down() error {
	return l.handle.LinkSetDown(l.Link)
}
