package docker

import (
	"context"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"

	"github.com/testground/testground/pkg/rpc"
)

var cli *client.Client

func init() {
	var err error
	cli, err = client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}

	cli.NegotiateAPIVersion(context.Background())

	rand.Seed(time.Now().UnixNano())
}

// pull an image from docker hub.
func pullImage(ctx context.Context, imageID string) error {
	options := types.ImagePullOptions{}
	c, err := cli.ImagePull(ctx, imageID, options)
	if err != nil {
		return err
	}
	return PipeOutput(c, os.Stderr)
}

// cleanup function which deletes a container
func deleteContainer(t *testing.T, containerID string) func() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	options := types.ContainerRemoveOptions{
		RemoveVolumes: true,
		RemoveLinks:   true,
		Force:         true,
	}
	return func() {
		err := cli.ContainerRemove(ctx, containerID, options)
		if err != nil {
			t.Error(err)
		}
	}
}

// creates a container. The image must already exist.
func createContainer(ctx context.Context, containerName string, imageID string) (string, error) {
	container_cfg := container.Config{
		Image: imageID,
	}
	host_cfg := container.HostConfig{}
	network_cfg := network.NetworkingConfig{}
	body, err := cli.ContainerCreate(ctx, &container_cfg, &host_cfg, &network_cfg, containerName)
	if err != nil {
		return "", err
	}
	return body.ID, err
}

// pull an image from docker hub.
// create a container with a randomized name
// Configure the container to be deleted when the test completes.
// return the container ID.
func pull_create_delete(t *testing.T, imageName string) (containerID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()
	containerName := t.Name() + "-" + strconv.FormatUint(rand.Uint64(), 16)

	err := pullImage(ctx, imageName)
	if err != nil {
		t.Fatal(err)
	}

	containerID, err = createContainer(ctx, containerName, imageName)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(deleteContainer(t, containerID))
	return
}

func TestFindImageFindsImages(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	imageName := "hello-world"
	ow := rpc.NewOutputWriter(nil, nil)
	err := pullImage(ctx, imageName)
	if err != nil {
		t.Error(err)
	}
	_, found, err := FindImage(ctx, ow, cli, imageName)
	if err != nil {
		t.Error(err)
	}

	if !found {
		t.Fail()
	}
}

func TestFindImageDoesNotFindNonExist(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	imageName := strconv.Itoa(rand.Int())
	ow := rpc.NewOutputWriter(nil, nil)

	_, found, err := FindImage(ctx, ow, cli, imageName)
	if err != nil {
		t.Error(err)
	}

	if found {
		t.Fail()
	}
}
