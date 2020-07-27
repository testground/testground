//+build linux

package sidecar

import (
	"fmt"

	"github.com/testground/testground/pkg/logging"
	"github.com/vishvananda/netlink"
)

type dockerRouting struct {
	enabled bool
	routes  []netlink.Route
}

func dockerRoutes(lnk link, netlinkHandle *netlink.Handle) (*dockerRouting, error) {
	routing := &dockerRouting{
		enabled: true,
		routes:  []netlink.Route{},
	}

	var defaultRoute netlink.Route

	// Get the current routes.
	linkRoutes, err := netlinkHandle.RouteList(lnk, netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("failed to list routes for link %s", lnk.Attrs().Name)
	}

	for _, route := range linkRoutes {
		if route.Dst == nil && route.Src == nil {
			defaultRoute = route
		} else {
			routing.routes = append(routing.routes, route)
		}
	}

	// the default route must go in the end so it is the last one to be
	// added when enabled external traffic
	routing.routes = append(routing.routes, defaultRoute)
	return routing, nil
}

func (routing *dockerRouting) disable(handle *netlink.Handle) error {
	if !routing.enabled {
		logging.S().Infow("external routing already disabled")
		return nil
	}

	for _, route := range routing.routes {
		if err := handle.RouteDel(&route); err != nil {
			return err
		}
	}

	routing.enabled = false
	logging.S().Infow("external routing disabled")
	return nil
}

func (routing *dockerRouting) enable(handle *netlink.Handle) error {
	if routing.enabled {
		logging.S().Infow("external routing already enabled")
		return nil
	}

	for _, route := range routing.routes {
		if err := handle.RouteAdd(&route); err != nil {
			return err
		}
	}

	routing.enabled = true
	logging.S().Infow("external routing enabled")
	return nil
}
