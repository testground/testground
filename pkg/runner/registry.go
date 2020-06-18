package runner

import (
	"context"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/docker"
	"github.com/testground/testground/pkg/rpc"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

func (c *ClusterK8sRunner) pushToDockerRegistry(ctx context.Context, ow *rpc.OutputWriter, client *client.Client, in *api.RunInput, ipo types.ImagePushOptions, uri string) error {
	for _, g := range in.Groups {
		tag := uri + ":" + g.ArtifactPath

		if _, ok := c.imagesLRU.Get(tag); ok {
			ow.Infow("image already pushed and tagged", "group_id", g.ID, "tag", tag)
			g.ArtifactPath = tag
			continue
		}

		ow.Infow("tagging image", "group_id", g.ID, "tag", tag)
		if err := client.ImageTag(ctx, g.ArtifactPath, tag); err != nil {
			return err
		}

		ow.Infow("pushing image for group", "group_id", g.ID, "tag", tag)
		rc, err := client.ImagePush(ctx, tag, ipo)
		if err != nil {
			return err
		}

		if _, err := docker.PipeOutput(rc, ow.StdoutWriter()); err != nil {
			return err
		}

		c.imagesLRU.Add(tag, struct{}{})

		// replace the artifact path by the pushed image.
		g.ArtifactPath = tag
		ow.Infow("pushed image for group", "group_id", g.ID, "tag", tag)
	}

	return nil
}
