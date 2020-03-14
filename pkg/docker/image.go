package docker

// Create docker images with a customized docker context.

import (
	"context"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"

	"go.uber.org/zap"
)

type EnsureImageOpts struct {
	Name     string
	BuildCtx string
}

// buildImage
// Use a Dockerfile and supporting files to build a docker image.
// Think `docker build /path/to/build`
func BuildImage(ctx context.Context, client *client.Client, opts *EnsureImageOpts) error {
	buildCtx, err := archive.TarWithOptions(opts.BuildCtx, &archive.TarOptions{})
	if err != nil {
		return err
	}
	defer buildCtx.Close()

	buildOpts := types.ImageBuildOptions{
		SuppressOutput: false,
		Remove:         true,
		ForceRemove:    true,
		PullParent:     true,
		Dockerfile:     "/Dockerfile",
		Tags:           []string{opts.Name},
	}
	buildResponse, err := client.ImageBuild(ctx, buildCtx, buildOpts)
	if err != nil {
		return err
	}
	defer buildResponse.Body.Close()
	return PipeOutput(buildResponse.Body, os.Stdout)
}

// EnsureImage
// Create a docker image from build context.
// If an image with the requested tag already exists, don't re-create it.
func EnsureImage(ctx context.Context, log *zap.SugaredLogger, client *client.Client, opts *EnsureImageOpts) (created bool, err error) {
	// Unfortunately we can't filter for RepoTags
	// Find out if we have any images with a RepoTag which matches the name of the image.
	// the RepoTag will be something like "name:latest" and I want to match any that have "name"
	listOpts := types.ImageListOptions{All: true}
	images, err := client.ImageList(ctx, listOpts)
	if err != nil {
		return false, err
	}
	for _, image := range images {
		for _, rt := range image.RepoTags {
			if strings.Contains(rt, opts.Name) {
				log.Info("found existing image")
				return false, nil
			}
		}
	}

	log.Info("creating new image")
	err = BuildImage(ctx, client, opts)
	if err != nil {
		log.Warn(err)
		return false, err
	}
	return true, err
}
