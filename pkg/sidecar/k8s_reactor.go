//go:build linux
// +build linux

package sidecar

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	gosync "sync"
	"time"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"

	"github.com/testground/testground/pkg/docker"
	"github.com/testground/testground/pkg/logging"

	"github.com/containernetworking/cni/libcni"
	"github.com/hashicorp/go-multierror"
	lru "github.com/hashicorp/golang-lru"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	controlNetworkIfname = "eth0"
	dataNetworkIfname    = "net1"
)

var (
	kubeDnsClusterIP = net.IPv4(10, 32, 0, 0)
)

type K8sReactor struct {
	gosync.Mutex

	client          sync.Client
	manager         *docker.Manager
	allowedServices []AllowedService
	runidsCache     *lru.Cache
}

func NewK8sReactor() (Reactor, error) {
	docker, err := docker.NewManager()
	if err != nil {
		return nil, err
	}

	client, err := sync.NewGenericClient(context.Background(), logging.S())
	if err != nil {
		return nil, err
	}

	cache, _ := lru.New(32)

	r := &K8sReactor{
		client:      client,
		manager:     docker,
		runidsCache: cache,
	}

	r.ResolveServices("constructor")

	return r, nil
}

func (d *K8sReactor) ResolveServices(runid string) {
	d.Lock()
	defer d.Unlock()

	if _, ok := d.runidsCache.Get(runid); ok {
		return
	}

	wantedServices := []struct {
		name string
		host string
	}{
		{
			"sync-service",
			os.Getenv(EnvSyncServiceHost),
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

	d.runidsCache.Add(runid, struct{}{})
	d.allowedServices = resolvedServices
}

func (d *K8sReactor) Handle(ctx context.Context, handler InstanceHandler) error {

	return d.manager.Watch(ctx, func(cctx context.Context, container *docker.ContainerRef) error {
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

func (d *K8sReactor) Close() error {
	var err *multierror.Error
	err = multierror.Append(err, d.manager.Close())
	err = multierror.Append(err, d.client.Close())
	return err.ErrorOrNil()
}

func (d *K8sReactor) manageContainer(ctx context.Context, container *docker.ContainerRef) (inst *Instance, err error) {
	// Get the state/config of the cluster
	info, err := container.Inspect(ctx)
	if err != nil {
		return nil, fmt.Errorf("inspect failed: %w", err)
	}

	if !info.State.Running {
		return nil, fmt.Errorf("container is not running: %s", container.ID)
	}

	// Construct the runtime environment
	params, err := runtime.ParseRunParams(info.Config.Env)
	if err != nil {
		return nil, fmt.Errorf("failed to parse run environment: %w", err)
	}

	if !params.TestSidecar {
		return nil, nil
	}

	podName, ok := info.Config.Labels["io.kubernetes.pod.name"]
	if !ok {
		return nil, fmt.Errorf("couldn't get pod name from container labels for: %s", container.ID)
	}

	// Resolve allowed services, so that we update network routes
	d.ResolveServices(params.TestRun)

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

	// Get a netlink handle.
	nshandle, err := netns.GetFromPid(info.State.Pid)
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
		netnsPath:       fmt.Sprintf("/proc/%d/ns/net", info.State.Pid),
		cninet:          cninet,
		container:       container,
		subnet:          runenv.TestSubnet.String(),
		nl:              netlinkHandle,
		activeLinks:     make(map[string]*k8sLink),
		externalRouting: map[string]*route{},
	}

	// Remove all routes but redis and the data subnet

	// We've found a control network (or some other network).
	controlLink, err := netlinkHandle.LinkByName(controlNetworkIfname)
	if err != nil {
		return nil, fmt.Errorf("failed to get link by name %s: %w", controlNetworkIfname, err)
	}

	controlLinkRoutes, err := netlinkHandle.RouteList(controlLink, netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("failed to list routes for control link %s", controlLink.Attrs().Name)
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

	return NewInstance(d.client, runenv, info.Config.Hostname, network)
}

func waitForPodRunningPhase(ctx context.Context, podName string) error {
	k8scfg, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		return fmt.Errorf("error in wait for pod running phase: %v", err)
	}

	k8sClientset, err := kubernetes.NewForConfig(k8scfg)
	if err != nil {
		return fmt.Errorf("error in wait for pod running phase: %v", err)
	}

	var phase string

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for pod context (pod name: %s) erred with: %w", podName, ctx.Err())
		default:
			if phase == "Running" {
				return nil
			}
			pod, err := k8sClientset.CoreV1().Pods("default").Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("error in wait for pod running phase: %v", err)
			}

			phase = string(pod.Status.Phase)

			time.Sleep(1 * time.Second)
		}
	}
}

type AllowedService struct {
	Name string
	IP   net.IP
}
