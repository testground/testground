package runner

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

// pushToAWSRegistry takes a RunInput and pushes all images to the AWS ECR registry.
// It then replaces the local artifact paths by the remote artifact paths.
func pushToAWSRegistry(ctx context.Context, ow *rpc.OutputWriter, client *client.Client, in *api.RunInput) error {
	ow.Infow("acquiring ECR authentication token")

	// Get a Docker registry authentication token from AWS ECR.
	auth, err := aws.ECR.GetAuthToken(in.EnvConfig.AWS)
	if err != nil {
		return err
	}
	ow.Infow("acquired ECR authentication token")

	// AWS ECR repository name is testground-<region>-<plan_name>.
	repo := fmt.Sprintf("testground-%s-%s", in.EnvConfig.AWS.Region, in.TestPlan)
	ow.Infow("ensuring ECR repository exists", "name", repo)
	// Ensure the repo exists, or create it. Get the full URI to the repo, so we
	// can tag images.
	uri, err := aws.ECR.EnsureRepository(in.EnvConfig.AWS, repo)
	if err != nil {
		return err
	}

	pushed := make(map[string]string, len(in.Groups))
	for _, g := range in.Groups {
		if uri, ok := pushed[g.ArtifactPath]; ok {
			ow.Infow("omitting push of previously pushed image", "group_id", g.ID, "tag", uri)
			g.ArtifactPath = uri
			continue
		}

		// Tag the image under the AWS ECR repository.
		tag := uri + ":" + g.ArtifactPath
		ow.Infow("tagging image", "tag", tag)
		if err = client.ImageTag(ctx, g.ArtifactPath, tag); err != nil {
			return err
		}

		// TODO for some reason, this push is way slower than the equivalent via the
		// docker CLI. Needs investigation.
		ow.Infow("pushing image for group", "group_id", g.ID, "tag", tag)
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

		pushed[g.ArtifactPath] = tag

		// replace the artifact path by the pushed image.
		g.ArtifactPath = tag
		ow.Infow("pushed image for group", "group_id", g.ID, "tag", tag)
	}

	return nil
}

func pushToDockerHubRegistry(ctx context.Context, ow *rpc.OutputWriter, client *client.Client, in *api.RunInput) error {
	uri := in.EnvConfig.DockerHub.Repo + "/testground"

	pushed := make(map[string]string, len(in.Groups))
	for _, g := range in.Groups {
		if uri, ok := pushed[g.ArtifactPath]; ok {
			ow.Infow("omitting push of previously pushed image", "group_id", g.ID, "tag", uri)
			g.ArtifactPath = uri
			continue
		}

		tag := uri + ":" + in.TestPlan
		ow.Infow("tagging image", "source", in.TestPlan, "repo", uri, "tag", tag)

		if err := client.ImageTag(ctx, g.ArtifactPath, tag); err != nil {
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

		ow.Infow("pushed image", "source", g.ArtifactPath, "tag", tag, "repo", uri)

		pushed[g.ArtifactPath] = tag
		g.ArtifactPath = tag

		// Pipe the docker output to stdout.
		if err := docker.PipeOutput(rc, ow.StdoutWriter()); err != nil {
			return err
		}
	}

	return nil
}
