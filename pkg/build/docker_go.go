package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/docker"
	"github.com/testground/testground/pkg/rpc"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
)

const (
	buildNetworkName = "testground-build"
)

var (
	_ api.Builder = &DockerGoBuilder{}

	dockerfileTmpl = template.Must(template.New("Dockerfile").Parse(DockerfileTemplate))
)

// DockerGoBuilder builds the test plan as a go-based container.
type DockerGoBuilder struct {
	proxyLk sync.Mutex
}

type DockerfileExtensions struct {
	PreModDownload  string `toml:"pre_mod_download"`
	PostModDownload string `toml:"post_mod_download"`
	PreSourceCopy   string `toml:"pre_source_copy"`
	PostSourceCopy  string `toml:"post_source_copy"`
	PreBuild        string `toml:"pre_build"`
	PostBuild       string `toml:"post_build"`
	PreRuntimeCopy  string `toml:"pre_runtime_copy"`
	PostRuntimeCopy string `toml:"post_runtime_copy"`
}

type DockerGoBuilderConfig struct {
	Enabled    bool
	GoVersion  string `toml:"go_version"`
	ModulePath string `toml:"module_path"`
	ExecPkg    string `toml:"exec_pkg"`
	FreshGomod bool   `toml:"fresh_gomod"`

	// GoProxyMode specifies one of "local", "direct", "remote".
	//
	//   * The "local" mode (default) will start a proxy container (if one
	//     doesn't exist yet) with bridge networking, and will configure the
	//     build to use that proxy.
	//   * The "direct" mode sets the `GOPROXY=direct` env var on the go build.
	//   * The "remote" mode specifies a custom proxy. The `GoProxyURL` field
	//     must be non-empty.
	GoProxyMode string `toml:"go_proxy_mode"`

	// GoProxyURL specifies the URL of the proxy when GoProxyMode = "custom".
	GoProxyURL string `toml:"go_proxy_url"`

	// RuntimeImage is the runtime image that the test plan binary will be
	// copied into. Defaults to busybox:1.31.1-glibc.
	RuntimeImage string `toml:"runtime_image"`

	// DockefileExtensions enables plans to inject custom Dockerfile directives.
	DockerfileExtensions DockerfileExtensions `toml:"dockerfile_extensions"`
}

type DockerfileTemplateVars struct {
	WithSDK              bool
	RuntimeImage         string
	DockerfileExtensions DockerfileExtensions
}

// Build builds a testplan written in Go and outputs a Docker container.
func (b *DockerGoBuilder) Build(ctx context.Context, in *api.BuildInput, ow *rpc.OutputWriter) (*api.BuildOutput, error) {
	cfg, ok := in.BuildConfig.(*DockerGoBuilderConfig)
	if !ok {
		return nil, fmt.Errorf("expected configuration type DockerGoBuilderConfig, was: %T", in.BuildConfig)
	}

	cliopts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}

	var (
		basesrc = in.BaseSrcPath
		plansrc = in.TestPlanSrcPath
		sdksrc  = in.SDKSrcPath

		cli, err = client.NewClientWithOpts(cliopts...)
	)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if err != nil {
		return nil, err
	}

	// Set up the go proxy wiring. This will start a goproxy container if
	// necessary, attaching it to the testground-build network.
	proxyURL, buildNetworkID, warn := b.setupGoProxy(ctx, ow, cli, cfg)
	if warn != nil {
		ow.Warnf("warning while setting up the go proxy: %s", warn)
	}

	// Write the Dockerfile.
	dockerfileDst := filepath.Join(basesrc, "Dockerfile")
	f, err := os.Create(dockerfileDst)
	if err != nil {
		return nil, fmt.Errorf("failed to create Dockerfile at %s: %w", dockerfileDst, err)
	}

	vars := &DockerfileTemplateVars{
		WithSDK:              sdksrc != "",
		RuntimeImage:         cfg.RuntimeImage,
		DockerfileExtensions: cfg.DockerfileExtensions,
	}

	if err = dockerfileTmpl.Execute(f, &vars); err != nil {
		return nil, fmt.Errorf("failed to execute Dockerfile template and/or write into file %s: %w", dockerfileDst, err)
	}

	if cfg.FreshGomod {
		for _, f := range []string{"go.mod", "go.sum"} {
			file := filepath.Join(plansrc, f)
			if _, err := os.Stat(file); !os.IsNotExist(err) {
				if err := os.Remove(file); err != nil {
					return nil, fmt.Errorf("cleanup failed; %w", err)
				}
			}
		}

		// Initialize a fresh go.mod file.
		cmd := exec.CommandContext(ctx, "go", "mod", "init", cfg.ModulePath)
		cmd.Dir = plansrc
		out, _ := cmd.CombinedOutput()
		if !strings.Contains(string(out), "creating new go.mod") {
			return nil, fmt.Errorf("unable to create go.mod; %s", out)
		}
	}

	// If we have version overrides, apply them.
	var replaces []string
	for mod, ver := range in.Dependencies {
		replaces = append(replaces, fmt.Sprintf("-replace=%s=%s@%s", mod, mod, ver))
	}

	// Inject replace directives for the SDK modules.
	if sdksrc != "" {
		replaces = append(replaces, "-replace=github.com/testground/sdk-go=../sdk")
	}

	if len(replaces) > 0 {
		// Write replace directives.
		cmd := exec.CommandContext(ctx, "go", append([]string{"mod", "edit"}, replaces...)...)
		cmd.Dir = plansrc
		if err = cmd.Run(); err != nil {
			out, _ := cmd.CombinedOutput()
			return nil, fmt.Errorf("unable to add replace directives to go.mod; %w; output: %s", err, string(out))
		}
	}

	// initial go build args.
	var args = map[string]*string{
		"GO_PROXY": &proxyURL,
	}

	if cfg.GoVersion != "" {
		args["GO_VERSION"] = &cfg.GoVersion
	}
	if cfg.ExecPkg != "" {
		args["TESTPLAN_EXEC_PKG"] = &cfg.ExecPkg
	}
	if cfg.RuntimeImage != "" {
		args["RUNTIME_IMAGE"] = &cfg.RuntimeImage
	}

	// set BUILD_TAGS arg if the user has provided selectors.
	if len(in.Selectors) > 0 {
		s := "-tags " + strings.Join(in.Selectors, ",")
		args["BUILD_TAGS"] = &s
	}

	// Make sure we are attached to the testground-build network
	// so the builder can make use of the goproxy container.
	opts := types.ImageBuildOptions{
		Tags:        []string{in.BuildID},
		BuildArgs:   args,
		NetworkMode: "host",
	}

	// If a docker network was created for the proxy, link it to the build container
	if buildNetworkID != "" {
		opts.NetworkMode = buildNetworkName
	}

	imageOpts := docker.BuildImageOpts{
		BuildCtx:  basesrc,
		BuildOpts: &opts,
	}

	buildStart := time.Now()

	err = docker.BuildImage(ctx, ow, cli, &imageOpts)
	if err != nil {
		return nil, fmt.Errorf("docker build failed: %w", err)
	}

	ow.Infow("build completed", "default_tag", fmt.Sprintf("%s:latest", in.BuildID), "took", time.Since(buildStart).Truncate(time.Second))

	imageID, err := docker.GetImageID(ctx, cli, in.BuildID)
	if err != nil {
		return nil, fmt.Errorf("couldnt get docker image id: %w", err)
	}

	ow.Infow("got docker image id", "image_id", imageID)

	deps, err := parseDependenciesFromDocker(ctx, ow, cli, imageID)
	if err != nil {
		return nil, fmt.Errorf("unable to list module dependencies; %w", err)
	}

	out := &api.BuildOutput{
		ArtifactPath: imageID,
		Dependencies: deps,
	}

	// Testplan image tag
	testplanImageTag := fmt.Sprintf("%s:%s", in.TestPlan, imageID)

	ow.Infow("tagging image", "image_id", imageID, "tag", testplanImageTag)
	if err = cli.ImageTag(ctx, out.ArtifactPath, testplanImageTag); err != nil {
		return out, err
	}

	return out, nil
}

func (*DockerGoBuilder) ID() string {
	return "docker:go"
}

func (*DockerGoBuilder) ConfigType() reflect.Type {
	return reflect.TypeOf(DockerGoBuilderConfig{})
}

func setupLocalGoProxyVol(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client) (*mount.Mount, error) {
	volumeOpts := docker.EnsureVolumeOpts{
		Name: "testground-goproxy-vol",
	}
	vol, _, err := docker.EnsureVolume(ctx, ow.SugaredLogger, cli, &volumeOpts)
	if err != nil {
		return nil, err
	}
	mnt := mount.Mount{
		Type:   mount.TypeVolume,
		Source: vol.Name,
		Target: "/go",
	}
	return &mnt, nil
}

// setupGoProxy sets up a goproxy container, if and only if the build
// configuration requires it.
//
// If an error occurs, it is reduced to a warning, and we fall back to direct
// mode (i.e. no proxy, not even Google's default one).
func (b *DockerGoBuilder) setupGoProxy(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, cfg *DockerGoBuilderConfig) (proxyURL string, buildNetworkID string, warn error) {
	// The testground-build network is used to connect build services (like the
	// GOPROXY) to the build container.
	b.proxyLk.Lock()
	defer b.proxyLk.Unlock()

	var mnt *mount.Mount

	switch strings.TrimSpace(cfg.GoProxyMode) {
	case "direct":
		proxyURL = "direct"
		ow.Debugw("[go_proxy_mode=direct] no goproxy container will be started")

	case "remote":
		if cfg.GoProxyURL == "" {
			warn = fmt.Errorf("[go_proxy_mode=remote] no proxy URL was supplied; falling back to go_proxy_mode=direct")
			proxyURL = "direct"
			break
		}

		proxyURL = cfg.GoProxyURL
		ow.Infof("[go_proxy_mode=remote] using url: %s", proxyURL)

	case "local":
		fallthrough

	default:
		var err error
		buildNetworkID, err = docker.EnsureBridgeNetwork(ctx, ow, cli, buildNetworkName, false)
		if err != nil {
			warn = fmt.Errorf("error while creating a testground-build network: %s; forcing direct proxy mode", err)
			cfg.GoProxyMode = "direct"
			proxyURL = cfg.GoProxyURL
			break
		}

		proxyURL = "http://testground-goproxy:8081"
		mnt, warn = setupLocalGoProxyVol(ctx, ow, cli)
		if warn != nil {
			proxyURL = "direct"
			warn = fmt.Errorf("encountered an error setting up the goproxy volueme; falling back to go_proxy_mode=direct; err: %w", warn)
			break
		}
		containerOpts := docker.EnsureContainerOpts{
			ContainerName: "testground-goproxy",
			ContainerConfig: &container.Config{
				Image: "goproxy/goproxy",
			},
			HostConfig: &container.HostConfig{
				Mounts:      []mount.Mount{*mnt},
				NetworkMode: container.NetworkMode(buildNetworkID),
			},
			ImageStrategy: docker.ImageStrategyPull,
		}
		_, _, warn = docker.EnsureContainerStarted(ctx, ow, cli, &containerOpts)
		if warn != nil {
			proxyURL = "direct"
			warn = fmt.Errorf("encountered an error when creating the goproxy container; falling back to go_proxy_mode=direct; err: %w", warn)
		}
	}
	return proxyURL, buildNetworkID, warn
}

const DockerfileTemplate = `
#:::
#::: BUILD CONTAINER
#:::

# GO_VERSION is the golang version this image will be built against.
ARG GO_VERSION=1.14.2

# This Dockerfile performs a multi-stage build and RUNTIME_IMAGE is the image
# onto which to copy the resulting binary. 
# Picking a different runtime base image from the build image allows us to
# slim down the deployable considerably.
#
# The user can override the runtime image by passing in the appropriate builder
# configuration option.
ARG RUNTIME_IMAGE=busybox:1.31.1-glibc

# Dynamically select the golang version.
FROM golang:${GO_VERSION}-buster AS builder

# TESTPLAN_EXEC_PKG is the executable package of the testplan to build.
# The image will build that package only.
ARG TESTPLAN_EXEC_PKG="."

# GO_PROXY is the go proxy that will be used, or direct by default.
ARG GO_PROXY=direct

# BUILD_TAGS is either nothing, or when expanded, it expands to "-tags <comma-separated build tags>"
ARG BUILD_TAGS

ENV TESTPLAN_EXEC_PKG ${TESTPLAN_EXEC_PKG}

# PLAN_DIR is the location containing the plan source inside the container.
ENV PLAN_DIR /plan/

# Copy only go.mod files and download deps, in order to leverage Docker caching.
COPY /plan/go.mod ${PLAN_DIR}

{{if .WithSDK}}
COPY /sdk/go.mod /sdk/go.mod
{{end}}

{{.DockerfileExtensions.PreModDownload}}

# Download deps.
RUN echo "Using go proxy: ${GO_PROXY}" \
    && cd ${PLAN_DIR} \
    && go env -w GOPROXY="${GO_PROXY}" \
    && go mod download

{{.DockerfileExtensions.PostModDownload}}

{{.DockerfileExtensions.PreSourceCopy}}

# Now copy the rest of the source and run the build.
COPY . /

{{.DockerfileExtensions.PostSourceCopy}}

{{.DockerfileExtensions.PreBuild}}

RUN cd ${PLAN_DIR} \
    && go env -w GOPROXY="${GO_PROXY}" \
    && GOOS=linux GOARCH=amd64 go build -o testplan ${BUILD_TAGS} ${TESTPLAN_EXEC_PKG}

{{.DockerfileExtensions.PostBuild}}

# Store module dependencies
RUN cd ${PLAN_DIR} \
  && go list -m all > /testground_dep_list

#:::
#::: RUNTIME CONTAINER
#:::

FROM ${RUNTIME_IMAGE} AS binary

{{.DockerfileExtensions.PreRuntimeCopy}}

COPY --from=builder /testground_dep_list /
COPY --from=builder /plan/testplan /

{{.DockerfileExtensions.PostRuntimeCopy}}

EXPOSE 6060
ENTRYPOINT [ "/testplan"]
`
