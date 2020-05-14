package build

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/aws"
	"github.com/testground/testground/pkg/docker"
	"github.com/testground/testground/pkg/rpc"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

func pushToAWSRegistry(ctx context.Context, ow *rpc.OutputWriter, client *client.Client, in *api.BuildInput, out *api.BuildOutput) error {
	// Get a Docker registry authentication token from AWS ECR.
	auth, err := aws.ECR.GetAuthToken(in.EnvConfig.AWS)
	if err != nil {
		return err
	}

	// AWS ECR repository name is testground-<region>-<plan_name>.
	repo := fmt.Sprintf("testground-%s-%s", in.EnvConfig.AWS.Region, in.TestPlan)

	// Ensure the repo exists, or create it. Get the full URI to the repo, so we
	// can tag images.
	uri, err := aws.ECR.EnsureRepository(in.EnvConfig.AWS, repo)
	if err != nil {
		return err
	}

	// Tag the image under the AWS ECR repository.
	tag := uri + ":" + in.BuildID
	ow.Infow("tagging image", "tag", tag)
	if err = client.ImageTag(ctx, out.ArtifactPath, tag); err != nil {
		return err
	}

	// TODO for some reason, this push is way slower than the equivalent via the
	// docker CLI. Needs investigation.
	ow.Infow("pushing image", "tag", tag)
	rc, err := client.ImagePush(ctx, tag, types.ImagePushOptions{
		RegistryAuth: aws.ECR.EncodeAuthToken(auth),
	})
	if err != nil {
		return err
	}

	// Pipe the docker output to stdout.
	if err := docker.PipeOutput(rc, ow.StdoutWriter()); err != nil {
		return err
	}

	// replace the artifact path by the pushed image.
	out.ArtifactPath = tag
	return nil
}

func pushToDockerHubRegistry(ctx context.Context, ow *rpc.OutputWriter, client *client.Client, in *api.BuildInput, out *api.BuildOutput) error {
	uri := in.EnvConfig.DockerHub.Repo + "/testground"

	tag := uri + ":" + in.BuildID
	ow.Infow("tagging image", "source", out.ArtifactPath, "repo", uri, "tag", tag)

	if err := client.ImageTag(ctx, out.ArtifactPath, tag); err != nil {
		return err
	}

	auth := types.AuthConfig{
		Username: in.EnvConfig.DockerHub.Username,
		Password: in.EnvConfig.DockerHub.AccessToken,
	}
	authBytes, err := json.Marshal(auth)
	if err != nil {
		return err
	}
	authBase64 := base64.URLEncoding.EncodeToString(authBytes)

	rc, err := client.ImagePush(ctx, uri, types.ImagePushOptions{
		RegistryAuth: authBase64,
	})
	if err != nil {
		return err
	}

	ow.Infow("pushed image", "source", out.ArtifactPath, "tag", tag, "repo", uri)

	// Pipe the docker output to stdout.
	if err := docker.PipeOutput(rc, ow.StdoutWriter()); err != nil {
		return err
	}

	// replace the artifact path by the pushed image.
	out.ArtifactPath = tag
	return nil
}
