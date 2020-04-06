package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"

	"github.com/ipfs/testground/pkg/rpc"
)

type BuildImageOpts struct {
	Name      string                   // reuired for EnsureImage
	BuildCtx  string                   // required
	BuildOpts *types.ImageBuildOptions // optional
}

func defaultBuildOptsFor(name string) *types.ImageBuildOptions {
	return &types.ImageBuildOptions{
		SuppressOutput: false,
		Remove:         true,
		ForceRemove:    true,
		PullParent:     true,
		Tags:           []string{name},
	}
}

// BuildImage builds a docker image from provided BuildImageOpts or a set of default options.
// If BuildImageOpts.BuildOpts is filled, these options will be passed to the docker client
// un-edited. In this case, BuildImageOpts.Name is unused when the image is created.
// When BuildImageOpts.BuildOpts has nil value, a default set of options will be constructed using
// the Name, and the constructed options are sent to the docker client.
// The build output is directed to stdout via PipeOutput.
func BuildImage(ctx context.Context, ow *rpc.OutputWriter, client *client.Client, opts *BuildImageOpts) error {
	buildCtx, err := archive.TarWithOptions(opts.BuildCtx, &archive.TarOptions{})
	if err != nil {
		return err
	}
	defer buildCtx.Close()

	var buildOpts *types.ImageBuildOptions
	if opts.BuildOpts == nil {
		buildOpts = defaultBuildOptsFor(opts.Name)
	} else {
		buildOpts = opts.BuildOpts
	}

	buildResponse, err := client.ImageBuild(ctx, buildCtx, *buildOpts)
	if err != nil {
		return err
	}
	defer buildResponse.Body.Close()
	return PipeOutput(buildResponse.Body, ow.StdoutWriter())
}

// EnsureImage builds an image only of one does not yet exist.
// This is a thin wrapper around BuildImage, and the same comments regarding the passed
// BuildImageOpts applies here. Returns a bool depending on whether the image had to be created and
// any errors that were encountered.
func EnsureImage(ctx context.Context, ow *rpc.OutputWriter, client *client.Client, opts *BuildImageOpts) (created bool, err error) {
	_, exists, err := FindImage(ctx, ow, client, opts.Name)
	if err != nil {
		return false, fmt.Errorf("failed while listing images: %w", err)
	}
	if exists {
		return false, nil
	}
	ow.Infof("image %s not found; building", opts.Name)
	err = BuildImage(ctx, ow, client, opts)
	if err != nil {
		ow.Warn(err)
		return false, err
	}
	return true, err
}

// FindImage looks for an image with name `name` in our local daemon.
//
// If found, it returns the image summary and true.
// If absent, it returns a nil image summary, and false.
// If an internal error occurs, it returns a nil image summary, false, and a non-nil error.
func FindImage(ctx context.Context, ow *rpc.OutputWriter, client *client.Client, name string) (*types.ImageSummary, bool, error) {
	// Find out if we have any images with a RepoTag which matches the name of the image.
	// the RepoTag will be something like "name:latest" and I want to match any that have "name"
	imageListOpts := types.ImageListOptions{All: true}
	images, err := client.ImageList(ctx, imageListOpts)
	if err != nil {
		ow.Errorw("retrieving list of images failed")
		return nil, false, err
	}
	for _, image := range images {
		for _, rt := range image.RepoTags {
			if strings.HasPrefix(rt, name) {
				ow.Infof("found existing image: %s", rt)
				return &image, true, nil
			}
		}
	}
	return nil, false, nil
}
