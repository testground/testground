package golang

import (
	"bytes"
	"context"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
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

	ccfg := &container.Config{
		Image: imageID,
		Tty:   true,
		Entrypoint: []string{
			"cat",
			"/testground_dep_list",
		},
	}

	// Create container
	res, err := cli.ContainerCreate(ctx, ccfg, nil, nil, "")
	if err != nil {
		return nil, err
	}

	// Start container
	err = cli.ContainerStart(ctx, res.ID, types.ContainerStartOptions{})
	if err != nil {
		return nil, err
	}

	// Get logs
	reader, err := cli.ContainerLogs(ctx, res.ID, types.ContainerLogsOptions{
		ShowStdout: true,
		Since:      "2019-01-01T00:00:00",
	})
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(reader)
	if err != nil {
		return nil, err
	}

	dependencies := buf.String()

	// Remove container
	err = cli.ContainerRemove(context.Background(), res.ID, types.ContainerRemoveOptions{Force: true})
	if err != nil {
		return nil, err
	}

	return parseDependencies(dependencies), nil
}
