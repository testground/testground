//+build linux

package sidecar

import (
	"fmt"
	"math"
	"net"
	"time"

	"github.com/vishvananda/netlink"
)

var (
	rootHandle    = netlink.MakeHandle(1, 0)
	defaultHandle = netlink.MakeHandle(1, 2)
)

var errMaxLatency = fmt.Errorf("latency can be at most %s", math.MaxUint32*time.Microsecond)

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
func (l *NetlinkLink) setBandwidth(idx uint16, rate uint64) error {
	bandwidthHandle, _ := handlesForIndex(idx)
	if rate == 0 {
		rate = math.MaxUint64
	}
	return l.handle.ClassChange(netlink.NewHtbClass(
		netlink.ClassAttrs{
			LinkIndex: l.Attrs().Index,
			Parent:    rootHandle,
			Handle:    bandwidthHandle,
		},
		netlink.HtbClassAttrs{
			Rate: rate,
		},
	))
}

// Sets the latency.
func (l *NetlinkLink) setLatency(idx uint16, latency time.Duration) error {
	targetLatency := latency.Microseconds()
	if targetLatency > math.MaxUint32 {
		return errMaxLatency
	}
	bandwidthHandle, latencyHandle := handlesForIndex(idx)
	return l.handle.QdiscChange(netlink.NewNetem(
		netlink.QdiscAttrs{
			LinkIndex: l.Attrs().Index,
			Parent:    bandwidthHandle,
			Handle:    latencyHandle,
		},
		netlink.NetemQdiscAttrs{
			Latency: uint32(targetLatency),
		},
	))
}

func (l *NetlinkLink) SetBandwidth(rate uint64) error {
	return l.setBandwidth(0, rate)
}

func (l *NetlinkLink) SetLatency(latency time.Duration) error {
	return l.setLatency(0, latency)
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
