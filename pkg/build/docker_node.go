package build

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/docker"
	"github.com/testground/testground/pkg/rpc"
)

const (
	DefaultNodeBuildBaseImage = "node:14.12.0-buster"
)

var (
	_ api.Builder = &DockerNodeBuilder{}
)

type DockerNodeBuilder struct{}

func (d DockerNodeBuilder) ID() string {
	return "docker:node"
}

func (d DockerNodeBuilder) Build(ctx context.Context, in *api.BuildInput, ow *rpc.OutputWriter) (*api.BuildOutput, error) {
	cfg, ok := in.BuildConfig.(*DockerNodeBuilderConfig)
	if !ok {
		return nil, fmt.Errorf("expected configuration type DockerNodeBuilderConfig, was: %T", in.BuildConfig)
	}

	cliopts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}

	var (
		basesrc  = in.UnpackedSources.BaseDir
		cli, err = client.NewClientWithOpts(cliopts...)
	)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	if err != nil {
		return nil, err
	}

	// Write the Dockerfile.
	dockerfileDst := filepath.Join(basesrc, "Dockerfile")
	err = ioutil.WriteFile(dockerfileDst, []byte(NodeDockerfileTemplate), 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create Dockerfile at %s: %w", dockerfileDst, err)
	}

	// fall back to default build base image, if one is not configured explicitly.
	if cfg.BaseImage == "" {
		cfg.BaseImage = DefaultNodeBuildBaseImage
	}

	// build args
	var args = map[string]*string{
		"BASE_IMAGE": &cfg.BaseImage,
	}

	opts := types.ImageBuildOptions{
		Tags:        []string{in.BuildID},
		BuildArgs:   args,
		NetworkMode: "host",
	}

	imageOpts := docker.BuildImageOpts{
		BuildCtx:  basesrc,
		BuildOpts: &opts,
	}

	buildStart := time.Now()

	_, err = docker.BuildImage(ctx, ow, cli, &imageOpts)
	if err != nil {
		return nil, fmt.Errorf("docker build failed: %w", err)
	}

	ow.Infow("build completed", "default_tag", fmt.Sprintf("%s:latest", in.BuildID), "took", time.Since(buildStart).Truncate(time.Second))

	imageID, err := docker.GetImageID(ctx, cli, in.BuildID)
	if err != nil {
		return nil, fmt.Errorf("couldnt get docker image id: %w", err)
	}

	ow.Infow("got docker image id", "image_id", imageID)

	out := &api.BuildOutput{
		ArtifactPath: imageID,
	}

	// Testplan image tag
	testplanImageTag := fmt.Sprintf("%s:%s", in.TestPlan, imageID)

	ow.Infow("tagging image", "image_id", imageID, "tag", testplanImageTag)
	if err = cli.ImageTag(ctx, out.ArtifactPath, testplanImageTag); err != nil {
		return out, err
	}

	return out, err
}

func (d DockerNodeBuilder) Purge(ctx context.Context, testplan string, ow *rpc.OutputWriter) error {
	return fmt.Errorf("purge not implemented for docker:node")
}

func (d DockerNodeBuilder) ConfigType() reflect.Type {
	return reflect.TypeOf(DockerNodeBuilderConfig{})
}

type DockerNodeBuilderConfig struct {
	Enabled   bool
	BaseImage string `toml:"base_image"`
}

const NodeDockerfileTemplate = `
ARG BASE_IMAGE
FROM ${BASE_IMAGE} AS builder
ENV PLAN_DIR /plan
WORKDIR /plan
# ENV LOG_LEVEL debug
COPY . /
RUN npm ci
EXPOSE 6060
ENTRYPOINT [ "node", "index.js"]
`
