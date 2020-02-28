package docker

import (
	"context"
	"errors"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"

	"go.uber.org/zap"
)

type EnsureContainerOpts struct {
	ContainerName      string
	ContainerConfig    *container.Config
	HostConfig         *container.HostConfig
	NetworkingConfig   *network.NetworkingConfig
	PullImageIfMissing bool
}

func CheckContainer(ctx context.Context, log *zap.SugaredLogger, cli *client.Client, name string) (container *types.ContainerJSON, err error) {
	log = log.With("container_name", name)

	log.Debug("checking state of container")

	// Check if a ${name} container exists.
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("name", name)),
	})
	if err != nil || len(containers) == 0 {
		return nil, err
	}

	c := containers[0]

	log.Debugw("container found", "container_id", c.ID, "state", c.State)

	ci, err := cli.ContainerInspect(ctx, c.ID)
	if err != nil {
		log.Errorw("inspecting container failed", "container_id", container.ID)
		return nil, err
	}

	return &ci, nil

}

// EnsureContainer ensures there's a container started of the specified kind.
func EnsureContainer(ctx context.Context, log *zap.SugaredLogger, cli *client.Client,
	opts *EnsureContainerOpts) (container *types.ContainerJSON, created bool, err error) {
	log = log.With("container_name", opts.ContainerName)

	log.Debug("checking state of container")

	ci, err := CheckContainer(ctx, log, cli, opts.ContainerName)
	if err != nil {
		return nil, false, err
	}

	if ci != nil && ci.State.Status != "running" {
		log.Infof("container isn't running; starting")

		err := cli.ContainerStart(ctx, ci.ID, types.ContainerStartOptions{})
		if err != nil {
			log.Errorw("starting container failed", "container_id", container.ID)
			return nil, false, err
		}

		ci, err = CheckContainer(ctx, log, cli, opts.ContainerName)
		return ci, false, err
	}

	log.Infow("container not found; creating")

	if opts.PullImageIfMissing {
		out, err := cli.ImagePull(ctx, opts.ContainerConfig.Image, types.ImagePullOptions{})
		if err != nil {
			return nil, false, err
		}

		if err := PipeOutput(out, os.Stdout); err != nil {
			return nil, false, err
		}
	} else {
		imageListOpts := types.ImageListOptions{
			All: true,
		}
		images, err := cli.ImageList(ctx, imageListOpts)
		if err != nil {
			log.Errorw("retrieving list of images failed")
			return nil, false, err
		}
		found := false
		for _, summary := range images {
			if len(summary.RepoTags) > 0 && summary.RepoTags[0] == opts.ContainerConfig.Image {
				found = true
				break
			}
		}
		if !found {
			log.Errorw("image not found", "image", opts.ContainerConfig.Image)
			err := errors.New("image not found")
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

	log.Infow("starting new container", "id", res.ID)

	err = cli.ContainerStart(ctx, res.ID, types.ContainerStartOptions{})
	if err == nil {
		log.Infow("started container", "id", res.ID)
	}

	c, err := cli.ContainerInspect(ctx, res.ID)
	if err == nil {
		log.Infow("started container", "id", res.ID)
	}

	return &c, true, err
}
