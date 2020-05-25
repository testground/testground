package runner

import (
	"context"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/docker"
	"github.com/testground/testground/pkg/rpc"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

func pushToDockerRegistry(ctx context.Context, ow *rpc.OutputWriter, client *client.Client, in *api.RunInput, ipo types.ImagePushOptions, uri string) error {
	pushed := make(map[string]string, len(in.Groups))
	for _, g := range in.Groups {
		if pap, ok := pushed[g.ArtifactPath]; ok {
			ow.Infow("omitting push of previously pushed image", "group_id", g.ID, "tag", pap)
			g.ArtifactPath = pap
			continue
		}

		tag := uri + ":" + in.TestPlan
		ow.Infow("tagging image", "source", in.TestPlan, "repo", uri, "tag", tag)

		if err := client.ImageTag(ctx, g.ArtifactPath, tag); err != nil {
			return err
		}

		ow.Infow("pushing image for group", "group_id", g.ID, "tag", tag)
		rc, err := client.ImagePush(ctx, tag, ipo)
		if err != nil {
			return err
		}

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
