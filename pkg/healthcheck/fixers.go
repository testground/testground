package healthcheck

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/ipfs/testground/pkg/docker"
	"github.com/ipfs/testground/pkg/rpc"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

// Fixer is a function that will be called to attempt to fix a failing check. It
// returns an optional message to present to the user, and error in case the fix
// failed.
type Fixer func() (msg string, err error)

// DefaultContainerFixer returns a Fixer, a method which when executed will ensure the named
// container exists with some default paramaters which are appropriate for infra containers.
// Unless containers require special consideration, this should be considered the sensible default
// fixer for docker containers.
func DefaultContainerFixer(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, opts *docker.EnsureContainerOpts) Fixer {
	// Make sure this container is running when the closure is executed.
	return func() (string, error) {
		_, created, err := docker.EnsureContainer(ctx, ow, cli, opts)
		if err != nil {
			return "failed to start container.", err
		}
		if created {
			return "container started", nil
		}
		return "container created.", nil
	}
}

// DockerImageFixer returns a Fixer, a method which when executed will create ensure a docker image
// is created. An error is returned if there is a failure to query or create the requested image.
func DockerImageFixer(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, opts *docker.BuildImageOpts) Fixer {
	return func() (string, error) {
		created, err := docker.EnsureImage(ctx, ow, cli, opts)
		if err != nil {
			return "failed to create custom image.", err
		}
		if created {
			return "custom image already existed.", nil
		}
		return "custom image created successfully.", nil
	}
}

// DockerNetworkFixer returns a Fixer, a method which when executed will create a docker network
// with the given name, provided it does not exist already.
func DockerNetworkFixer(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, networkID string, netcfg network.IPAMConfig) Fixer {
	return func() (string, error) {
		_, err := docker.EnsureBridgeNetwork(ctx, ow, cli, networkID, false, netcfg)
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

// And returns a Fixer. This method takes Fixers as its parameters. When the returned Fixer is
// executed. If any of the Fixers included encounter an error, no further action is taken and the
// error is returned. Use when there is a set multiple fixes which should be executed to mitigate a
// single failed Checker.
func And(fixers ...Fixer) Fixer {
	return func() (string, error) {
		for _, fxr := range fixers {
			msg, err := fxr()
			if err != nil {
				return msg, err
			}
		}
		return "all fixes mitigated.", nil
	}
}

// And returns a Fixer. This method takes Fixers as its parameters. When the returned Fixer is
// executed. As soon as the first fixer returns without error, execution stops and a successful
// status is returned. An error is returned if all passed Fixers return an error. Use when any of
// several Fixes could be used to mitigate a failed Checker.
func Or(fixers ...Fixer) Fixer {
	return func() (string, error) {
		for _, fxr := range fixers {
			msg, err := fxr()
			if err == nil {
				return msg, err
			}
		}
		return "all fixes failed.", fmt.Errorf("all fixes failed.")
	}
}
