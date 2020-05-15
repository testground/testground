//+build linux

package sidecar

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"

	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/sidecar/containerd"

	"github.com/containernetworking/cni/libcni"
	"github.com/hashicorp/go-multierror"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"github.com/vishvananda/netns"
)

type K8sContainerdReactor struct {
	client          *sync.Client
	manager         *containerd.Manager
	allowedServices []AllowedService
}

func NewK8sContainerdReactor() (Reactor, error) {
	wantedServices := []struct {
		name string
		host string
	}{
		{
			"redis",
			os.Getenv(EnvRedisHost),
		},
		{
			"influxdb",
			os.Getenv(EnvInfluxdbHost),
		},
	}

	var resolvedServices []AllowedService
	for _, s := range wantedServices {
		if s.host == "" {
			continue
		}
		ip, err := net.ResolveIPAddr("ip4", s.host)
		if err != nil {
			logging.S().Warnw("failed to resolve host", "service", s.name, "host", s.host, "err", err.Error())
			continue
		}
		resolvedServices = append(resolvedServices, AllowedService{s.name, ip.IP})
	}

	manager := containerd.NewManager()

	client, err := sync.NewGenericClient(context.Background(), logging.S())
	if err != nil {
		return nil, err
	}

	// sidecar nodes perform Redis GC.
	client.EnableBackgroundGC(nil)

	return &K8sContainerdReactor{
		client:          client,
		manager:         manager,
		allowedServices: resolvedServices,
	}, nil
}

func (d *K8sContainerdReactor) Handle(ctx context.Context, handler InstanceHandler) error {
	return d.manager.Watch(ctx, func(cctx context.Context, container *containerd.ContainerRef) error {
		logging.S().Debugw("got container", "container", container.ID)
		inst, err := d.manageContainer(cctx, container)
		if err != nil {
			return fmt.Errorf("failed to initialise the container: %w", err)
		}
		if inst == nil {
			logging.S().Debugw("ignoring container", "container", container.ID)
			return nil
		}
		logging.S().Debugw("managing container", "container", container.ID)
		err = handler(cctx, inst)
		if err != nil {
			return fmt.Errorf("container worker failed: %w", err)
		}
		return nil
	})
}

func (d *K8sContainerdReactor) Close() error {
	var err *multierror.Error
	err = multierror.Append(err, d.manager.Close())
	err = multierror.Append(err, d.client.Close())
	return err.ErrorOrNil()
}

func (d *K8sContainerdReactor) manageContainer(ctx context.Context, container *containerd.ContainerRef) (inst *Instance, err error) {
	// Get the state/config of the cluster
	isrunning, err := container.IsRunning(ctx)
	if err != nil {
		return nil, fmt.Errorf("inspect failed: %w", err)
	}

	if !isrunning {
		return nil, fmt.Errorf("container is not running: %s", container.ID)
	}

	// Construct the runtime environment
	env, err := container.Env(ctx)
	if err != nil {
		return nil, fmt.Errorf("env failed: %w", err)
	}
	params, err := runtime.ParseRunParams(env)
	if err != nil {
		return nil, fmt.Errorf("failed to parse run environment: %w", err)
	}

	if !params.TestSidecar {
		return nil, nil
	}

	labels, err := container.Labels(ctx)
	if err != nil {
		return nil, fmt.Errorf("labels failed: %w", err)
	}
	podName, ok := labels["io.kubernetes.pod.name"]
	if !ok {
		return nil, fmt.Errorf("couldn't get pod name from container labels for: %s", container.ID)
	}

	err = waitForPodRunningPhase(ctx, podName)
	if err != nil {
		return nil, err
	}

	// Remove the TestOutputsPath. We can't store anything from the sidecar.
	params.TestOutputsPath = ""
	runenv := runtime.NewRunEnv(*params)

	//////////////////
	//  NETWORKING  //
	//////////////////

	// Initialise CNI config
	cninet := libcni.NewCNIConfig(filepath.SplitList("/host/opt/cni/bin"), nil)

	// Get PID
	pid, err := container.Pid(ctx)
	if err != nil {
		return nil, fmt.Errorf("pid failed: %w", err)
	}
	// Get a netlink handle.
	nshandle, err := netns.GetFromPid(pid)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup the net namespace: %s", err)
	}
	defer nshandle.Close()

	netlinkHandle, err := netlink.NewHandleAt(nshandle)
	if err != nil {
		return nil, fmt.Errorf("failed to get handle to network namespace: %w", err)
	}

	defer func() {
		if err != nil {
			netlinkHandle.Delete()
		}
	}()

	// Finally, construct the network manager.
	network := &K8sNetwork{
		netnsPath:   fmt.Sprintf("/proc/%d/ns/net", pid),
		cninet:      cninet,
		container:   container,
		subnet:      runenv.TestSubnet.String(),
		nl:          netlinkHandle,
		activeLinks: make(map[string]*k8sLink),
	}

	// Remove all routes but redis and the data subnet

	// We've found a control network (or some other network).
	controlLink, err := netlinkHandle.LinkByName(controlNetworkIfname)
	if err != nil {
		return nil, fmt.Errorf("failed to get link by name %s: %w", controlNetworkIfname, err)
	}

	var servicesIPs []net.IP

	for _, s := range d.allowedServices {
		// Get the routes to redis, influxdb, etc... We need to keep these.
		r, err := getServiceRoute(netlinkHandle, s.IP)
		if err != nil {
			return nil, fmt.Errorf("cant get route to %s ; %s: %s", s.IP, s.Name, err)
		}
		logging.S().Debugw("got service route", "route.Src", r.Src, "route.Dst", r.Dst, "gw", r.Gw.String(), "container", container.ID)

		servicesIPs = append(servicesIPs, r.Dst.IP)
	}

	controlLinkRoutes, err := netlinkHandle.RouteList(controlLink, netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("failed to list routes for control link %s", controlLink.Attrs().Name)
	}

	routesToBeDeleted := []netlink.Route{}

	// Remove the original routes
	for _, route := range controlLinkRoutes {
		routeDst := "nil"
		if route.Dst != nil {
			routeDst = route.Dst.String()
		}

		logging.S().Debugw("inspecting controlLink route", "route.Src", route.Src, "route.Dst", routeDst, "gw", route.Gw, "container", container.ID)

		if route.Dst != nil && route.Dst.String() == podCidr {
			logging.S().Debugw("marking for deletion podCidr dst route", "route.Src", route.Src, "route.Dst", routeDst, "gw", route.Gw, "container", container.ID)
			routesToBeDeleted = append(routesToBeDeleted, route)
			continue
		}

		if route.Dst != nil {
			for _, serviceIP := range servicesIPs {
				if route.Dst.Contains(serviceIP) {
					newroute := route
					newroute.Dst = &net.IPNet{
						IP:   serviceIP,
						Mask: net.CIDRMask(32, 32),
					}

					logging.S().Debugw("adding service route", "route.Src", newroute.Src, "route.Dst", newroute.Dst.String(), "gw", newroute.Gw, "container", container.ID)
					if err := netlinkHandle.RouteAdd(&newroute); err != nil {
						logging.S().Debugw("failed to add route while restricting gw route", "container", container.ID, "err", err.Error())
					} else {
						logging.S().Debugw("successfully added route", "route.Src", newroute.Src, "route.Dst", newroute.Dst.String(), "gw", newroute.Gw, "container", container.ID)
					}
				}
			}

			logging.S().Debugw("marking for deletion some dst route", "route.Src", route.Src, "route.Dst", routeDst, "gw", route.Gw, "container", container.ID)
			routesToBeDeleted = append(routesToBeDeleted, route)
			continue
		}

		logging.S().Debugw("marking for deletion random route", "route.Src", route.Src, "route.Dst", routeDst, "gw", route.Gw, "container", container.ID)
		routesToBeDeleted = append(routesToBeDeleted, route)
	}

	// Adding DNS route
	for _, route := range controlLinkRoutes {
		if route.Dst == nil && route.Src == nil {
			// if default route, get the gw and add a route for DNS
			dnsRoute := route
			dnsRoute.Src = nil
			dnsRoute.Dst = &net.IPNet{
				IP:   kubeDnsClusterIP,
				Mask: net.CIDRMask(32, 32),
			}

			logging.S().Debugw("adding dns route", "container", container.ID)
			if err := netlinkHandle.RouteAdd(&dnsRoute); err != nil {
				return nil, fmt.Errorf("failed to add dns route to pod: %v", err)
			}
		}
	}

	for _, r := range routesToBeDeleted {
		// Don't route to the default route. Blackhole these routes.
		bh := netlink.Route{
			Dst:  r.Dst,
			Type: nl.FR_ACT_BLACKHOLE,
		}
		routeDst := "nil"
		if r.Dst != nil {
			routeDst = r.Dst.String()
		}

		logging.S().Debugw("really removing route", "route.Src", r.Src, "route.Dst", routeDst, "gw", r.Gw, "container", container.ID)
		if err := netlinkHandle.RouteDel(&r); err != nil {
			logging.S().Warnw("failed to really delete route", "route.Src", r.Src, "gw", r.Gw, "route.Dst", routeDst, "container", container.ID, "err", err.Error())
		}
		if err := netlinkHandle.RouteAdd(&bh); err != nil {
			logging.S().Warnw("failed to add blackhole route")
		}
	}

	// Get Hostname
	hostname, err := container.Hostname(ctx)
	if err != nil {
		return nil, fmt.Errorf("hostname failed: %w", err)
	}
	return NewInstance(d.client, runenv, hostname, network)
}
