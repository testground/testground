package runner

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"
	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/docker"
	"github.com/ipfs/testground/pkg/rpc"
)

type Checker func() (bool, error)
type Fixer func() error

// HealthcheckHelper is a strategy interface for runners.
// Each runner may have required elements -- infrastructure, etc. which should be checked prior to
// running plans. Individual checks are registered to the HealthcheckHelper using the Enlist()
// method. With all of the checks enlisted, execute the checks, and optionally fixes, using the
// RunChecks() method. The details of how the checks are performed is implementation specific.
// Typically, the checker and fixer passed to the enlist method will be closures. These methods will
// be called when RunChecks is executed.
type HealthcheckHelper interface {
	Enlist(name string, c Checker, f Fixer)
	RunChecks(ctx context.Context, fix bool) error
}

type toDoElement struct {
	Name    string
	Checker Checker
	Fixer   Fixer
}

// SequentialHealthcheckHelper implements HealthcheckHelper. Runchecks runs each check and fix
// sequentially, in the order they are Enlist()'ed.
type SequentialHealthcheckHelper struct {
	toDo   []*toDoElement
	report *api.HealthcheckReport
}

func (hh *SequentialHealthcheckHelper) Enlist(name string, c Checker, f Fixer) {
	hh.toDo = append(hh.toDo, &toDoElement{name, c, f})
}

func (hh *SequentialHealthcheckHelper) RunChecks(ctx context.Context, fix bool) error {
	for _, li := range hh.toDo {
		// Check succeeds.
		succeed, err := li.Checker()
		if err != nil {
			return err
		}
		if succeed {
			hh.report.Checks = append(hh.report.Checks, api.HealthcheckItem{
				Name:    li.Name,
				Status:  api.HealthcheckStatusOK,
				Message: fmt.Sprintf("%s: OK", li.Name),
			})
			continue
		}
		// Checker failed, Append the failure to the check report
		hh.report.Checks = append(hh.report.Checks, api.HealthcheckItem{
			Name:    li.Name,
			Status:  api.HealthcheckStatusFailed,
			Message: fmt.Sprintf("%s: FAILED. Fixing: %t", li.Name, fix),
		})
		// Attempt fix if fix is enabled.
		// The fix might result in a failure, a successful recovery.
		fixhc := api.HealthcheckItem{Name: li.Name}
		if fix {
			err := li.Fixer()
			if err != nil {
				// Oh no! the fix failed.
				fixhc.Status = api.HealthcheckStatusFailed
				fixhc.Message = fmt.Sprintf("%s FAILED: %v", li.Name, err)
			} else {
				// Fix succeeded.
				fixhc.Status = api.HealthcheckStatusOK
				fixhc.Message = fmt.Sprintf("%s RECOVERED", li.Name)
			}
		} else {
			// don't attempt to fix.
			fixhc.Status = api.HealthcheckStatusOmitted
			fixhc.Message = fmt.Sprintf("%s recovery not attempted.", li.Name)
		}
		// Fill the report with fix information.
		hh.report.Fixes = append(hh.report.Fixes, fixhc)
	}
	return nil
}

// DefaultContainerChecker returns a Checker, a method which when executed will check for the
// existance of the container. This should be considered a sensible default for checking whether
// docker containers are started.
func DefaultContainerChecker(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, name string) Checker {
	return func() (bool, error) {
		ci, err := docker.CheckContainer(ctx, ow, cli, name)
		if err != nil || ci == nil {
			return false, err
		}
		return ci.State.Running, nil
	}
}

// Options used by the DefaultContainerFixer and the CustomContainerFixer
// ContainerName and ImageName are requred fields.
// PortSpecs and NetworkID are used internally to construct a HostConfig, either the one provided
// or the a default HostConfig will be constructed.
// HostConfig is a docker container config object, and is not normally required. Use this when
// additional capabilities or usunusal configuration is required.
// Cmds is a slice of string options passed to the container. Use this if the container takes
// command-line parameters.
type ContainerFixerOpts struct {
	ContainerName string
	ImageName     string
	NetworkID     string
	PortSpecs     []string
	Pull          bool
	HostConfig    *container.HostConfig
	Cmds          []string
}

// DefaultContainerFixer returns a Fixer, a method which when executed will ensure the named
// container exists with some default paramaters which are appropriate for infra containers.
// Unless containers require special consideration, this should be considered the sensible default
// fixer for docker containers.
func DefaultContainerFixer(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, opts *ContainerFixerOpts) Fixer {
	// Docker host configuration.
	// https://godoc.org/github.com/docker/docker/api/types/container#HostConfig
	var hostConfig container.HostConfig
	// If we have not provided a HostConfig, create a default one.
	if opts.HostConfig == nil {
		//	if reflect.DeepEqual(opts.HostConfig, container.HostConfig{}) {
		hostConfig = container.HostConfig{
			Resources: container.Resources{
				Ulimits: []*units.Ulimit{
					{Name: "nofile", Hard: InfraMaxFilesUlimit, Soft: InfraMaxFilesUlimit},
				},
			},
		}
	} else {
		hostConfig = *opts.HostConfig
	}
	hostConfig.NetworkMode = container.NetworkMode(opts.NetworkID)
	// Try to parse the portSpecs, but if we can't, fall back to using random host port assignments.
	// the portSpec should be in the format ip:public:private/proto
	_, portBindings, err := nat.ParsePortSpecs(opts.PortSpecs)
	if err != nil {
		// Fall back to picking a random port.
		hostConfig.PublishAllPorts = true
	} else {
		hostConfig.PortBindings = portBindings
	}

	// Configuration for the container:
	containerConfig := container.Config{
		Image: opts.ImageName,
		Cmd:   opts.Cmds,
	}

	ensure := docker.EnsureContainerOpts{
		ContainerName:      opts.ContainerName,
		ContainerConfig:    &containerConfig,
		HostConfig:         &hostConfig,
		PullImageIfMissing: opts.Pull,
	}

	// Make sure this container is running when the closure is executed.
	return func() error {
		_, _, err := docker.EnsureContainer(ctx, ow, cli, &ensure)
		return err
	}
}

// CustomContainerFixer returns a Fixer, a method which when executed will ensure the named
// container exists. Unlike the DefaultContainerFixer, a custom image may be built for the
// container.
func CustomContainerFixer(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, buildCtx string, opts *ContainerFixerOpts) Fixer {
	return func() error {
		_, err := docker.EnsureImage(ctx,
			ow,
			cli,
			&docker.BuildImageOpts{
				Name:     opts.ImageName,
				BuildCtx: buildCtx,
			})
		if err != nil {
			return err
		}
		return DefaultContainerFixer(ctx, ow, cli, opts)()

	}
}

// DockerNetworkChecker returns a Checker, a method which when executed will verify a docker network
// exists with the passed networkID as its name.
func DockerNetworkChecker(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, networkID string) Checker {
	return func() (bool, error) {
		networks, err := docker.CheckBridgeNetwork(ctx, ow, cli, networkID)
		if err != nil {
			ow.Errorf("encountered an error while checking for network %s, %v", networkID, err)
			return false, err
		}
		return len(networks) > 0, nil
	}
}

// DockerNetworkFixer returns a Fixer, a method which when executed will create a docker network
// with the given name, provided it does not exist already.
func DockerNetworkFixer(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, networkID string) Fixer {
	return func() error {
		_, err := docker.EnsureBridgeNetwork(
			ctx,
			ow, cli,
			networkID,
			// making internal=false enables us to expose ports to the host (e.g.
			// pprof and prometheus). by itself, it would allow the container to
			// access the Internet, and therefore would break isolation, but since
			// we have sidecar overriding the default Docker ip routes, and
			// suppressing such traffic, we're safe.
			false,
			network.IPAMConfig{
				Subnet:  controlSubnet,
				Gateway: controlGateway,
			},
		)
		return err
	}
}

// DialableChecker returns a Checker, a method which when executed will tell us whether a
// port is dialable. For TCP sockets, a false return could mean the network is unreachable,
// or that a TCP socket is closed. For UDP sockets, being connectionless, may return a false
// positive if the network is reachable.
func DialableChecker(protocol string, address string) Checker {
	return func() (bool, error) {
		_, err := net.Dial(protocol, address)
		return err == nil, err
	}
}

// CommandStartFixer returns a Fixer, a method which when executed will start an executable
// with the given parameters. Uses os/exec to start the command. Cancelling the passed context
// will stop the executable.
func CommandStartFixer(ctx context.Context, cmd string, args ...string) Fixer {
	return func() error {
		cmd := exec.CommandContext(ctx, cmd, args...)
		return cmd.Start()
	}
}

// DirExistsChecker returns a Checker, a method which when executed will check whether a director
// exists. A true value means the directory exists. A false value means it does not exist, or
// that the path does not point to a directory.
func DirExistsChecker(path string) Checker {
	return func() (bool, error) {
		fi, err := os.Stat(path)
		if err != nil {
			// ErrExist is the error we expect to see (and handle with DirExistsFixer)
			// Any other kind of error will be returned.
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, err
		}
		return fi.IsDir(), nil
	}
}

// DirExistsFixer returns a Fixer, a method which when executed will create a directory and
// any parent directories as appropriate.
func DirExistsFixer(path string) Fixer {
	return func() error {
		return os.MkdirAll(path, os.ModeDir)
	}
}
