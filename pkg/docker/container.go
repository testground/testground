package docker

import (
	"context"
	"errors"
	"os"

	"github.com/ipfs/testground/pkg/util"

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

// EnsureContainer ensures there's a container started of the specified kind.
func EnsureContainer(ctx context.Context, log *zap.SugaredLogger, cli *client.Client,
	opts *EnsureContainerOpts) (container *types.ContainerJSON, created bool, err error) {
	log = log.With("containerName", opts.ContainerName)

	log.Debug("checking state of container")

	// Check if a ${containerName} container exists.
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("name", opts.ContainerName)),
	})
	if err != nil {
		return nil, false, err
	}

	if len(containers) > 0 {
		container := containers[0]

		log.Infow("container found", "containerId", container.ID, "state", container.State)

		switch container.State {
		case "running":
		default:
			log.Infof("container isn't running; starting")
			err := cli.ContainerStart(ctx, container.ID, types.ContainerStartOptions{})
			if err != nil {
				log.Errorw("starting container failed", "containerId", container.ID)
				return nil, false, err
			}
		}

		c, err := cli.ContainerInspect(ctx, container.ID)
		if err != nil {
			log.Errorw("inspecting container failed", "containerId", container.ID)
			return nil, false, err
		}

		return &c, false, nil
	}

	log.Infow("container not found; creating")

	if opts.PullImageIfMissing {
		out, err := cli.ImagePull(ctx, opts.ContainerConfig.Image, types.ImagePullOptions{})
		if err != nil {
			return nil, false, err
		}

		if err := util.PipeDockerOutput(out, os.Stdout); err != nil {
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
			if summary.RepoTags[0] == opts.ContainerConfig.Image {
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
