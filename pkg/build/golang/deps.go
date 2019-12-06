package golang

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"go.uber.org/zap"
)

func parseDependencies(raw string) map[string]string {
	rawModules := strings.Split(raw, "\n")
	modules := map[string]string{}

	for _, module := range rawModules {
		module = strings.TrimSpace(module)
		if module == "" {
			continue
		}

		parts := strings.SplitN(module, " ", 2)
		path := strings.TrimSpace(parts[0])
		version := ""

		if len(parts) == 2 {
			version = strings.TrimSpace(parts[1])
		}

		modules[path] = version
	}

	return modules
}

func parseDependenciesFromDocker(ctx context.Context, log *zap.SugaredLogger, cli *client.Client, imageID string) (map[string]string, error) {
	res, err := cli.ContainerCreate(ctx, &container.Config{Image: imageID}, nil, nil, "")
	if err != nil {
		return nil, err
	}

	defer func() {
		err = cli.ContainerRemove(context.Background(), res.ID, types.ContainerRemoveOptions{Force: true})
		if err != nil {
			log.Warnf("error while removing container %s: %v", res.ID, err)
		}
	}()

	tar, _, err := cli.CopyFromContainer(ctx, res.ID, "/testground_dep_list")
	if err != nil {
		return nil, err
	}

	dir, err := ioutil.TempDir("", res.ID)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	err = archive.Untar(tar, dir, &archive.TarOptions{NoLchown: true})
	if err != nil {
		return nil, err
	}

	deps, err := ioutil.ReadFile(path.Join(dir, "testground_dep_list"))
	if err != nil {
		return nil, err
	}
	return parseDependencies(string(deps)), nil
}
