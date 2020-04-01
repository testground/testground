package healthcheck

import (
	"context"
	"os"
	"os/exec"

	"github.com/ipfs/testground/pkg/docker"
	"github.com/ipfs/testground/pkg/rpc"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

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
			//			Resources: container.Resources{
			//				Ulimits: []*units.Ulimit{
			//					{Name: "nofile", Hard: InfraMaxFilesUlimit, Soft: InfraMaxFilesUlimit},
			//				},
			//			},
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
	return func() (string, error) {
		_, _, err := docker.EnsureContainer(ctx, ow, cli, &ensure)
		if err != nil {
			return "failed to start container.", err
		}
		return "container created.", nil
	}
}

// CustomContainerFixer returns a Fixer, a method which when executed will ensure the named
// container exists. Unlike the DefaultContainerFixer, a custom image may be built for the
// container.
func CustomContainerFixer(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, buildCtx string, opts *ContainerFixerOpts) Fixer {
	return func() (string, error) {
		_, err := docker.EnsureImage(ctx,
			ow,
			cli,
			&docker.BuildImageOpts{
				Name:     opts.ImageName,
				BuildCtx: buildCtx,
			})
		if err != nil {
			return "failed to create custom image.", err
		}
		return DefaultContainerFixer(ctx, ow, cli, opts)()
	}
}

// DockerNetworkFixer returns a Fixer, a method which when executed will create a docker network
// with the given name, provided it does not exist already.
func DockerNetworkFixer(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, networkID string) Fixer {
	return func() (string, error) {
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
			//			network.IPAMConfig{
			//				Subnet:  controlSubnet,
			//				Gateway: controlGateway,
			//			},
		)
		if err != nil {
			return "could not create network.", err
		}
		return "network created.", nil
	}
}

// CommandStartFixer returns a Fixer, a method which when executed will start an executable
// with the given parameters. Uses os/exec to start the command. Cancelling the passed context
// will stop the executable.
func CommandStartFixer(ctx context.Context, cmd string, args ...string) Fixer {
	return func() (string, error) {
		cmd := exec.CommandContext(ctx, cmd, args...)
		err := cmd.Start()
		if err != nil {
			return "command did not start successfully.", err
		}
		return "command started successfully.", nil
	}
}

// DirExistsFixer returns a Fixer, a method which when executed will create a directory and
// any parent directories as appropriate.
func DirExistsFixer(path string) Fixer {
	return func() (string, error) {
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return "directory not created successfully.", err
		}
		return "directory created successfully.", nil
	}
}
