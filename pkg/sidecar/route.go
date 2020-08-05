//+build linux

package sidecar

import (
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/testground/sdk-go/network"

	"github.com/testground/testground/pkg/logging"
	"github.com/vishvananda/netlink"
)

type route struct {
	enabled bool
	routes  []netlink.Route
}

func getDockerRoutes(lnk netlink.Link, netlinkHandle *netlink.Handle) (*route, error) {
	routing := &route{
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

func getK8sRoutes(lnk netlink.Link, netlinkHandle *netlink.Handle) (*route, error) {
	routing := &route{
		enabled: true,
		routes:  []netlink.Route{},
	}

	// Get the current routes.
	linkRoutes, err := netlinkHandle.RouteList(lnk, netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("failed to list routes for link %s", lnk.Attrs().Name)
	}

	for _, route := range linkRoutes {
		if route.Dst == nil && route.Src == nil {
			routing.routes = append(routing.routes, route)
		}
	}

	return routing, nil
}

func (routing *route) disable(handle *netlink.Handle) error {
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

func (routing *route) enable(handle *netlink.Handle) error {
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

func handleRoutingPolicy(routes map[string]*route, policy network.RoutingPolicyType, handle *netlink.Handle) error {
	var err *multierror.Error

	for _, routing := range routes {
		switch policy {
		case network.AllowAll:
			err = multierror.Append(err, routing.enable(handle))
		case network.DenyAll:
			fallthrough
		default:
			err = multierror.Append(err, routing.disable(handle))
		}
	}

	return err.ErrorOrNil()
}
