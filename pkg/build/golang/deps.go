package golang

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
)

func parseDependencies(raw string) map[string]string {
	rawModules := strings.Split(raw, "\n")
	modules := map[string]string{}

	for _, module := range rawModules {
		module = strings.TrimSpace(module)
		if module == "" {
			continue
		}

		parts := strings.Split(module, " ")
		path := strings.TrimSpace(parts[0])
		version := ""

		if len(parts) == 2 {
			version = strings.TrimSpace(parts[1])
		}

		modules[path] = version
	}

	return modules
}

func parseDependenciesFromDocker(cli *client.Client, imageID string) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create container
	res, err := cli.ContainerCreate(ctx, &container.Config{Image: imageID}, nil, nil, "")
	if err != nil {
		return nil, err
	}

	defer func() {
		// Remove container
		err = cli.ContainerRemove(context.Background(), res.ID, types.ContainerRemoveOptions{Force: true})
		if err != nil {
			fmt.Printf("error while removing container %s: %v", res.ID, err)
		}
	}()

	// Copy file from container
	tar, _, err := cli.CopyFromContainer(ctx, res.ID, "/testground_dep_list")
	if err != nil {
		return nil, err
	}

	dir, err := ioutil.TempDir("", res.ID)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	// Unpack the file
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
