package docker

import (
	"context"
	"errors"
	"fmt"

	"github.com/testground/testground/pkg/rpc"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/hashicorp/go-multierror"
)

type ImageStrategy int

const (
	ImageStrategyNone ImageStrategy = iota
	ImageStrategyPull
	ImageStrategyBuild
)

type EnsureContainerOpts struct {
	ContainerName    string
	ContainerConfig  *container.Config
	HostConfig       *container.HostConfig
	NetworkingConfig *network.NetworkingConfig
	ImageStrategy    ImageStrategy
	BuildImageOpts   *BuildImageOpts
}

func CheckContainer(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, name string) (container *types.ContainerJSON, err error) {
	ow = ow.With("container_name", name)

	ow.Debug("checking state of container")

	// filter regex; container names have a preceding slash. Newer versions of
	// the Docker daemon appear to test filters against slash-prefixed and
	// non-slash-prefixed versions of the container name; older versions appear
	// not to do this trickery. Since `docker inspect <container_id> -f
	// '{{.Name}}'` returns a slash-prefixed name, we assume that's the
	// canonical name. To be compatible with a wide range of Docker daemon
	// versions, we choose to compare against that.
	//
	// More info:
	// https://github.com/testground/testground/pull/782#issuecomment-608422093.
	exactMatch := fmt.Sprintf("^/%s$", name)
	// Check if a ${name} container exists.
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("name", exactMatch)),
	})
	if err != nil || len(containers) == 0 {
		return nil, err
	}

	c := containers[0]

	ow.Debugw("container found", "container_id", c.ID, "state", c.State)

	ci, err := cli.ContainerInspect(ctx, c.ID)
	if err != nil {
		ow.Errorw("inspecting container failed", "container_id", c.ID)
		return nil, err
	}

	return &ci, nil

}

// EnsureContainerStarted ensures there's a container started of the specified
// kind, resorting to building it if necessary and so indicated.
func EnsureContainerStarted(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, opts *EnsureContainerOpts) (container *types.ContainerJSON, created bool, err error) {
	log := ow.With("container_name", opts.ContainerName)

	log.Debug("checking state of container")

	ci, err := CheckContainer(ctx, ow, cli, opts.ContainerName)
	if err != nil {
		return nil, false, err
	}

	if ci != nil {
		if ci.State.Status == "running" {
			log.Info("container is already running")
			return ci, false, err
		}
		log.Info("container isn't running; starting")

		err := cli.ContainerStart(ctx, ci.ID, types.ContainerStartOptions{})
		if err != nil {
			log.Errorw("starting container failed", "container_name", opts.ContainerName, "error", err)
			return nil, false, err
		}

		ci, err = CheckContainer(ctx, ow, cli, opts.ContainerName)
		return ci, false, err
	}

	log.Infow("container not found; creating")

	switch opts.ImageStrategy {
	case ImageStrategyNone:
		_, found, err := FindImage(ctx, log, cli, opts.ContainerConfig.Image)
		if err != nil {
			log.Warnw("failed to check if image exists", "image", opts.ContainerConfig.Image, "error", err)
			return nil, false, err
		}
		if !found {
			log.Warnw("image not found", "image", opts.ContainerConfig.Image)
			err := errors.New("image not found")
			return nil, false, err
		}

	case ImageStrategyPull:
		out, err := cli.ImagePull(ctx, opts.ContainerConfig.Image, types.ImagePullOptions{})
		if err != nil {
			return nil, false, err
		}
		if _, err := PipeOutput(out, ow.StdoutWriter()); err != nil {
			return nil, false, err
		}

	case ImageStrategyBuild:
		_, err := EnsureImage(ctx, ow, cli, opts.BuildImageOpts)
		if err != nil {
			err = fmt.Errorf("failed to check/build image: %w", err)
			return nil, false, err
		}
	}

	res, err := cli.ContainerCreate(ctx,
		opts.ContainerConfig,
		opts.HostConfig,
		opts.NetworkingConfig,
		opts.ContainerName,
	)

	if err != nil {
		return nil, false, err
	}

	log.Infow("created container", "id", res.ID)
	log.Infow("starting container", "id", res.ID)

	err = cli.ContainerStart(ctx, res.ID, types.ContainerStartOptions{})
	if err != nil {
		return nil, false, err
	}

	log.Infow("started container", "id", res.ID)

	c, err := cli.ContainerInspect(ctx, res.ID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to inspect container: %w", err)
	}

	return &c, true, err
}

// DeleteContainers deletes a set of containers in parallel, using a ratelimit
// of 16 concurrent delete requests. If a deletion fails, it does not
// short-circuit. Instead, it accumulates errors and returns an multierror.
func DeleteContainers(cli *client.Client, ow *rpc.OutputWriter, ids []string) (err error) {
	ow.Infow("deleting containers", "ids", ids)

	ratelimit := make(chan struct{}, 16)

	errs := make(chan error)
	for _, id := range ids {
		go func(id string) {
			ratelimit <- struct{}{}
			defer func() { <-ratelimit }()

			ow.Infow("deleting container", "id", id)
			errs <- cli.ContainerRemove(context.Background(), id, types.ContainerRemoveOptions{Force: true})
		}(id)
	}

	var merr *multierror.Error
	for i := 0; i < len(ids); i++ {
		if err := <-errs; err != nil {
			ow.Errorw("failed while deleting container", "error", err)
			merr = multierror.Append(merr, <-errs)
		}
	}
	close(errs)
	return merr.ErrorOrNil()
}
