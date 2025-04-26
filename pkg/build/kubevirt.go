package build

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"time"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/docker"
	"github.com/testground/testground/pkg/rpc"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
)

var (
	_ api.Builder = &DockerKubevirtBuilder{}
)

type DockerKubevirtBuilder struct {
	Enabled bool
}

type DockerKubevirtBuilderConfig struct {
	Path      string             `toml:"path" default:"./"`
	BuildArgs map[string]*string `toml:"build_args"`
	// qcow2_url is the URL to the base QCOW2 image to use for the VM disk. Must be set.
	QCOW2URL string `toml:"qcow2_url"`
}

func (b *DockerKubevirtBuilder) Build(ctx context.Context, in *api.BuildInput, ow *rpc.OutputWriter) (*api.BuildOutput, error) {
	cfg, ok := in.BuildConfig.(*DockerKubevirtBuilderConfig)
	if !ok {
		return nil, fmt.Errorf("expected configuration type DockerKubevirtBuilderConfig, was: %T", in.BuildConfig)
	}

	if cfg.QCOW2URL == "" {
		return nil, fmt.Errorf("qcow2_url is missing or empty in the build config; please set a valid URL for the base QCOW2 image (e.g. https://download.cirros-cloud.net/0.6.2/cirros-0.6.2-x86_64-disk.img)")
	}

	cliopts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}

	var (
		basesrc  = in.UnpackedSources.BaseDir
		cli, err = client.NewClientWithOpts(cliopts...)
	)
	if err != nil {
		return nil, err
	}

	planPath := cfg.Path
	basePathForPlan := path.Join("/plan", planPath)

	if cfg.BuildArgs == nil {
		cfg.BuildArgs = make(map[string]*string)
	}

	if _, ok = cfg.BuildArgs["PLAN_PATH"]; !ok {
		cfg.BuildArgs["PLAN_PATH"] = &cfg.Path
	}

	// 1. Build the testplan binary using the Dockerfile (like docker:generic)
	opts := types.ImageBuildOptions{
		Tags:        []string{in.BuildID},
		BuildArgs:   cfg.BuildArgs,
		NetworkMode: "host",
		Dockerfile:  filepath.Join(basePathForPlan, "Dockerfile"),
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

	// Use a temp dir for all intermediate files
	tmpDir, err := os.MkdirTemp("", "kubevirt-builder-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// 2. Extract the built binary from the Docker image using Docker API
	testplanBin := filepath.Join(tmpDir, "testplan")
	{
		ow.Infow("extracting testplan binary from docker image using Docker API")
		// Create container
		containerResp, err := cli.ContainerCreate(ctx, &container.Config{
			Image: imageID,
		}, nil, nil, "")
		if err != nil {
			return nil, fmt.Errorf("docker API container create failed: %w", err)
		}
		// Copy file from container
		reader, _, err := cli.CopyFromContainer(ctx, containerResp.ID, "/testplan")
		if err != nil {
			return nil, fmt.Errorf("docker API copy from container failed: %w", err)
		}
		defer reader.Close()
		// The tar archive contains the file; extract it
		tarReader := tar.NewReader(reader)
		for {
			hdr, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("error reading tar from container: %w", err)
			}
			if hdr.Name == "testplan" || filepath.Base(hdr.Name) == "testplan" {
				f, err := os.Create(testplanBin)
				if err != nil {
					return nil, fmt.Errorf("failed to create testplan: %w", err)
				}
				if _, err := io.Copy(f, tarReader); err != nil {
					f.Close()
					return nil, fmt.Errorf("failed to copy testplan: %w", err)
				}
				f.Close()
				break
			}
		}
		// Remove container
		_ = cli.ContainerRemove(ctx, containerResp.ID, types.ContainerRemoveOptions{Force: true})
	}

	// 3. Download the base QCOW2 image (still using curl for now)
	qcow2Path := filepath.Join(tmpDir, "base.qcow2")
	{
		ow.Infow("downloading base QCOW2 image", "url", cfg.QCOW2URL)
		cmdCurl := exec.CommandContext(ctx, "curl", "-L", "-o", qcow2Path, cfg.QCOW2URL)
		if out, err := cmdCurl.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("failed to download QCOW2 image: %w\n%s", err, string(out))
		}
	}

	// 4. Inject the testplan binary into the QCOW2 image using virt-customize
	{
		ow.Infow("injecting testplan binary into QCOW2 image (in-place)")
		cmdVirt := exec.CommandContext(ctx, "virt-customize", "-a", qcow2Path, "--copy-in", fmt.Sprintf("%s:/root", testplanBin))
		if out, err := cmdVirt.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("virt-customize failed: %w\n%s", err, string(out))
		}
		// Debug: print file size and type of base.qcow2
		fi, err := os.Stat(qcow2Path)
		if err != nil {
			ow.Warnw("could not stat base.qcow2 after virt-customize", "error", err)
		} else {
			ow.Infow("base.qcow2 file info", "size", fi.Size())
		}
		cmdFile := exec.CommandContext(ctx, "file", qcow2Path)
		if out, err := cmdFile.CombinedOutput(); err != nil {
			ow.Warnw("could not run file on base.qcow2", "error", err)
		} else {
			ow.Infow("base.qcow2 file type", "output", string(out))
		}
		// Ensure base.qcow2 is world-readable (mode 0644)
		if err := os.Chmod(qcow2Path, 0644); err != nil {
			ow.Warnw("could not set permissions on base.qcow2", "error", err)
		} else {
			ow.Infow("set permissions on base.qcow2 to 0644")
		}
	}

	// 5. Write Dockerfile.kubevirt dynamically
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile.kubevirt")
	{
		ow.Infow("writing Dockerfile.kubevirt")
		dockerfileContent := "FROM kubevirt/container-disk-v1alpha\nADD base.qcow2 /disk\n"
		if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644); err != nil {
			return nil, fmt.Errorf("failed to write Dockerfile.kubevirt: %w", err)
		}
	}

	// 6. Build and tag the resulting image using Docker API
	finalTag := fmt.Sprintf("%s-kubevirt", in.BuildID)
	{
		ow.Infow("building final kubevirt containerDisk image using Docker API")
		buildCtx, err := archive.TarWithOptions(tmpDir, &archive.TarOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create build context tar: %w", err)
		}
		defer buildCtx.Close()
		buildOpts := types.ImageBuildOptions{
			Tags:       []string{finalTag},
			Dockerfile: "Dockerfile.kubevirt",
			Remove:     true,
		}
		resp, err := cli.ImageBuild(ctx, buildCtx, buildOpts)
		if err != nil {
			return nil, fmt.Errorf("docker API image build failed: %w", err)
		}
		io.Copy(io.Discard, resp.Body) // Drain output
		resp.Body.Close()
		// Get image ID
		imageID, err := docker.GetImageID(ctx, cli, finalTag)
		if err != nil {
			return nil, fmt.Errorf("failed to get final image ID: %w", err)
		}
		ow.Infow("final kubevirt image built", "image_id", imageID)
		return &api.BuildOutput{
			ArtifactPath: imageID,
		}, nil
	}
}

func (*DockerKubevirtBuilder) ID() string {
	return "docker:kubevirt"
}

func (*DockerKubevirtBuilder) ConfigType() reflect.Type {
	return reflect.TypeOf(DockerKubevirtBuilderConfig{})
}

func (*DockerKubevirtBuilder) Purge(ctx context.Context, testplan string, ow *rpc.OutputWriter) error {
	return fmt.Errorf("purge not implemented for docker:kubevirt")
}
