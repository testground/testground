//go:build linux
// +build linux

package sidecar

import (
	"context"
	"fmt"
	"net"
	"os"
	gosync "sync"
	"time"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
	"github.com/testground/testground/pkg/docker"
	"github.com/testground/testground/pkg/logging"

	"github.com/hashicorp/go-multierror"
	lru "github.com/hashicorp/golang-lru"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
)

type KubevirtReactor struct {
	gosync.Mutex

	client          sync.Client
	manager         *docker.Manager
	allowedServices []AllowedService
	runidsCache     *lru.Cache
	kvClient        kubecli.KubevirtClient
}

func NewKubevirtReactor() (Reactor, error) {
	docker, err := docker.NewManager()
	if err != nil {
		return nil, err
	}

	client, err := sync.NewGenericClient(context.Background(), logging.S())
	if err != nil {
		return nil, err
	}

	cache, _ := lru.New(32)

	// Initialize KubeVirt client
	k8sCfg, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		return nil, fmt.Errorf("could not start k8s client from config: %v", err)
	}

	kvClient, err := kubecli.GetKubevirtClientFromRESTConfig(k8sCfg)
	if err != nil {
		return nil, fmt.Errorf("could not create kubevirt client: %v", err)
	}

	r := &KubevirtReactor{
		client:      client,
		manager:     docker,
		runidsCache: cache,
		kvClient:    kvClient,
	}

	r.ResolveServices("constructor")

	return r, nil
}

func (d *KubevirtReactor) ResolveServices(runid string) {
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

func (d *KubevirtReactor) Handle(ctx context.Context, handler InstanceHandler) error {
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

func (d *KubevirtReactor) Close() error {
	var err *multierror.Error
	err = multierror.Append(err, d.manager.Close())
	err = multierror.Append(err, d.client.Close())
	return err.ErrorOrNil()
}

func (d *KubevirtReactor) manageContainer(ctx context.Context, container *docker.ContainerRef) (inst *Instance, err error) {
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

	vmName, ok := info.Config.Labels["kubevirt.io/domain"]
	if !ok {
		return nil, fmt.Errorf("couldn't get VM name from container labels for: %s", container.ID)
	}

	// Resolve allowed services, so that we update network routes
	d.ResolveServices(params.TestRun)

	err = waitForVMRunningPhase(ctx, d.kvClient, vmName)
	if err != nil {
		return nil, err
	}

	// Remove the TestOutputsPath. We can't store anything from the sidecar.
	params.TestOutputsPath = ""
	runenv := runtime.NewRunEnv(*params)

	//////////////////
	//  NETWORKING  //
	//////////////////

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
		cninet:          nil, // We don't use CNI in KubeVirt
		container:       container,
		subnet:          runenv.TestSubnet.String(),
		nl:              netlinkHandle,
		activeLinks:     make(map[string]*k8sLink),
		externalRouting: map[string]*route{},
	}

	return NewInstance(d.client, runenv, info.Config.Hostname, network)
}

func waitForVMRunningPhase(ctx context.Context, kvClient kubecli.KubevirtClient, vmName string) error {
	var phase kubevirtv1.VirtualMachineInstancePhase

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for VM context (VM name: %s) erred with: %w", vmName, ctx.Err())
		default:
			if phase == kubevirtv1.Running {
				return nil
			}
			vm, err := kvClient.VirtualMachineInstance("testground-infra").Get(ctx, vmName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("error in wait for VM running phase: %v", err)
			}

			phase = vm.Status.Phase

			time.Sleep(1 * time.Second)
		}
	}
}
