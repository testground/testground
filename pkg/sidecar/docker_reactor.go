//go:build linux
// +build linux

package sidecar

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	gosync "sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/hashicorp/go-multierror"
	lru "github.com/hashicorp/golang-lru"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
	"github.com/testground/testground/pkg/docker"
	"github.com/testground/testground/pkg/logging"
)

// PublicAddr points to an IP address in the public range. It helps us discover
// the IP address of the gateway (i.e. the Docker host) on the control network
// (the learned route will be via the control network because, at this point,
// the only network that's attached to the container is the control network).
//
// Sidecar doesn't whitelist traffic to public addresses, but it special-cases
// traffic between the container and the host, so that pprof, metrics and other
// ports can be exposed to the Docker host.
var PublicAddr = net.ParseIP("1.1.1.1")

type DockerReactor struct {
	client sync.Client
	gosync.Mutex
	servicesRoutes []net.IP
	manager        *docker.Manager
	runidsCache    *lru.Cache
}

func NewDockerReactor() (Reactor, error) {
	docker, err := docker.NewManager()
	if err != nil {
		return nil, err
	}

	client, err := sync.NewGenericClient(context.Background(), logging.S())
	if err != nil {
		return nil, err
	}

	cache, _ := lru.New(32)

	r := &DockerReactor{
		client:      client,
		manager:     docker,
		runidsCache: cache,
	}

	r.ResolveServices("constructor")

	return r, nil
}

func (d *DockerReactor) ResolveServices(runid string) {
	d.Lock()
	defer d.Unlock()

	if _, ok := d.runidsCache.Get(runid); ok {
		return
	}

	wantedRoutes := []string{
		os.Getenv(EnvRedisHost), // NOTE: kept for backwards compatibility with older SDKs.
		os.Getenv(EnvSyncServiceHost),
		os.Getenv(EnvInfluxdbHost),
	}

	additionalHosts := strings.Split(os.Getenv(EnvAdditionalHosts), ",")
	logging.S().Infow("additional hosts", "hosts", os.Getenv(EnvAdditionalHosts))
	wantedRoutes = append(wantedRoutes, additionalHosts...)

	var resolvedRoutes []net.IP
	for _, route := range wantedRoutes {
		if route == "" {
			continue
		}
		ip, err := net.ResolveIPAddr("ip4", route)
		if err != nil {
			logging.S().Warnw("failed to resolve host", "host", route, "err", err.Error())
			continue
		}
		logging.S().Infow("resolved route to host", "host", route, "ip", ip.String())
		resolvedRoutes = append(resolvedRoutes, ip.IP)
	}

	d.runidsCache.Add(runid, struct{}{})
	d.servicesRoutes = resolvedRoutes
}

func (d *DockerReactor) Handle(globalctx context.Context, handler InstanceHandler) error {
	return d.manager.Watch(globalctx, func(ctx context.Context, container *docker.ContainerRef) error {
		logging.S().Debugw("got container", "container", container.ID)
		inst, err := d.handleContainer(ctx, container)
		if err != nil {
			return fmt.Errorf("failed to initialise the container: %w", err)
		}
		if inst == nil {
			logging.S().Debugw("ignoring container", "container", container.ID)
			return nil
		}

		err = handler(ctx, inst)
		if err != nil {
			return fmt.Errorf("container worker failed: %w", err)
		}
		return nil
	}, "testground.run_id")
}

func (d *DockerReactor) Close() error {
	var err *multierror.Error
	err = multierror.Append(err, d.manager.Close())
	err = multierror.Append(err, d.client.Close())
	return err.ErrorOrNil()
}

func (d *DockerReactor) handleContainer(ctx context.Context, container *docker.ContainerRef) (inst *Instance, err error) {
	// Get the state/config of the cluster
	info, err := container.Inspect(ctx)
	if err != nil {
		return nil, fmt.Errorf("inspect failed: %w", err)
	}

	if !info.State.Running {
		return nil, fmt.Errorf("not running")
	}

	// Construct the runtime environment
	params, err := runtime.ParseRunParams(info.Config.Env)
	if err != nil {
		return nil, fmt.Errorf("failed to parse run environment: %w", err)
	}

	// Not using the sidecar, ignore this container.
	if !params.TestSidecar {
		return nil, nil
	}

	if strings.Contains(info.Name, "mkdir-outputs") {
		return nil, nil
	}

	logging.S().Debugw("handle container", "name", info.Name, "image", info.Image)

	// Resolve allowed services, so that we update network routes
	d.ResolveServices(params.TestRun)

	// Remove the TestOutputsPath. We can't store anything from the sidecar.
	params.TestOutputsPath = ""
	runenv := runtime.NewRunEnv(*params)

	//////////////////
	//  NETWORKING  //
	//////////////////

	// TODO: cache this?
	networks, err := container.Manager.NetworkList(ctx, types.NetworkListOptions{
		Filters: filters.NewArgs(
			filters.Arg(
				"label",
				"testground.run_id="+info.Config.Labels["testground.run_id"],
			),
		),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	// Get a netlink handle.
	nshandle, netlinkHandle, err := getNetworkHandlers(info.State.Pid)
	if err != nil {
		return nil, err
	}
	defer func() {
		nshandle.Close()

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
		container:       container,
		activeLinks:     make(map[string]*dockerLink, len(info.NetworkSettings.Networks)),
		availableLinks:  make(map[string]string, len(networks)),
		externalRouting: map[string]*route{},
		nl:              netlinkHandle,
	}

	// Retrieve control routes.
	controlRoutes, err := getControlRoutes(d.servicesRoutes, container.ID, netlinkHandle)
	if err != nil {
		return nil, err
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

		// Get all current routes and store them.
		routes, err := getDockerRoutes(link, netlinkHandle)
		if err != nil {
			return nil, fmt.Errorf("failed to enumerate routes: %w", err)
		}
		network.externalRouting[id] = routes

		// Add learned routes plan containers so they can reach  the testground infra on the control network.
		for _, route := range controlRoutes {
			if route.LinkIndex != link.Attrs().Index {
				continue
			}
			if err := netlinkHandle.RouteAdd(&route); err != nil {
				return nil, fmt.Errorf("failed to add new route: %w", err)
			}
		}
	}

	return NewInstance(d.client, runenv, info.Config.Hostname, network)
}

func getNetworkHandlers(pid int) (netns.NsHandle, *netlink.Handle, error) {
	// Get a netlink handle.
	nshandle, err := netns.GetFromPid(pid)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to lookup the net namespace: %s", err)
	}

	netlinkHandle, err := netlink.NewHandleAt(nshandle)
	if err != nil {
		nshandle.Close()
		return 0, nil, fmt.Errorf("failed to get handle to network namespace: %w", err)
	}

	return nshandle, netlinkHandle, nil
}

func getControlRoutes(servicesRoutes []net.IP, id string, netlinkHandle *netlink.Handle) ([]netlink.Route, error) {
	var controlRoutes []netlink.Route
	for _, route := range servicesRoutes {
		nlroutes, err := netlinkHandle.RouteGet(route)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve route %s: %w", route, err)
		}
		controlRoutes = append(controlRoutes, nlroutes...)
	}

	// Get the route to a public address. We will NOT be whitelisting traffic to
	// public IPs, but this helps us discover the IP address of the Docker host
	// on the control network. See the godoc on the PublicAddr var for more
	// info.
	pub, err := netlinkHandle.RouteGet(PublicAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve route for %s: %w", PublicAddr, err)
	}

	switch {
	case len(pub) == 0:
		logging.S().Warnw("failed to discover gateway/host address; no routes to public IPs", "container_id", id)
	case pub[0].Gw == nil:
		logging.S().Warnw("failed to discover gateway/host address; gateway is nil", "route", pub[0], "container_id", id)
	default:
		hostRoutes, err := netlinkHandle.RouteGet(pub[0].Gw)
		if err != nil {
			logging.S().Warnw("failed to add route for gateway/host address", "error", err, "route", pub[0], "container_id", id)
			break
		}
		logging.S().Infow("successfully resolved route to host", "container_id", id)
		controlRoutes = append(controlRoutes, hostRoutes...)
	}

	return controlRoutes, nil
}
