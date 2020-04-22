package healthcheck

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/testground/testground/pkg/docker"
	"github.com/testground/testground/pkg/rpc"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

// StartContainer returns a Fixer that starts the specified container if it
// exists, potentially acquiring the image first via the supplied image
// strategy, if the image itself is absent.
func StartContainer(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, opts *docker.EnsureContainerOpts) Fixer {
	return func() (string, error) {
		_, created, err := docker.EnsureContainerStarted(ctx, ow, cli, opts)
		if err != nil {
			return "failed to start container.", err
		}
		if created {
			return "container started", nil
		}
		return "container created.", nil
	}
}

// BuildImage returns a Fixer that builds the provided image if it doesn't
// exist yet.
func BuildImage(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, opts *docker.BuildImageOpts) Fixer {
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

// CreateNetwork returns a Fixer that creates a Docker bridge network with
// supplied ID and characteristics.
func CreateNetwork(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, networkID string, netcfg network.IPAMConfig) Fixer {
	return func() (string, error) {
		_, err := docker.EnsureBridgeNetwork(ctx, ow, cli, networkID, false, netcfg)
		if err != nil {
			return "could not create network.", err
		}
		return "network created.", nil
	}
}

// StartCommand returns a Fixer that starts the given process, under the
// supplied context, via os/exec.CommandContext.
func StartCommand(ctx context.Context, cmd string, args ...string) Fixer {
	return func() (string, error) {
		cmd := exec.CommandContext(ctx, cmd, args...)
		err := cmd.Start()
		if err != nil {
			return "command did not start successfully.", err
		}
		return "command started successfully.", nil
	}
}

// CreateDirectory returns a Fixer that creates the specified directory and any
// parent directories as appropriate.
func CreateDirectory(path string) Fixer {
	return func() (string, error) {
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return "directory not created successfully.", err
		}
		return "directory created successfully.", nil
	}
}

// NotImplemented is a placeholder Fixer which always returns successfully.
func NotImplemented() Fixer {
	return func() (string, error) {
		return "fix is not implemented", nil
	}
}

// And returns a Fixer that executes all fixes sequentially, short-circuiting at
// the first error.
func And(fixers ...Fixer) Fixer {
	return func() (string, error) {
		for _, fxr := range fixers {
			msg, err := fxr()
			if err != nil {
				return msg, err
			}
		}
		return "all fixes applied.", nil
	}
}

// Or returns a Fixer that short-circuits on the first success, or errors if all
// fixes failed.
func Or(fixers ...Fixer) Fixer {
	// TODO probably want to use a multierror to accumulate all errors.
	return func() (string, error) {
		for _, fxr := range fixers {
			msg, err := fxr()
			if err == nil {
				return msg, err
			}
		}
		return "all fixes failed.", fmt.Errorf("all fixes failed")
	}
}
