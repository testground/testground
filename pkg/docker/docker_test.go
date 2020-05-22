package docker_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/require"

	"github.com/testground/testground/pkg/docker"
	"github.com/testground/testground/pkg/rpc/rpctest"
)

var cli *client.Client

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Pull an image (to ensure it exists) then make sure FindImage can find it.
func TestFindImageFindsImages(t *testing.T) {
	initDockerClientOrSkip(t)

	_, ow := rpctest.NewRecordedOutputWriter(t.Name())
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	imageName := "hello-world"
	err := pullImage(ctx, imageName)
	require.NoError(t, err)

	_, found, err := docker.FindImage(ctx, ow, cli, imageName)
	require.NoError(t, err)
	require.NotNil(t, found)
}

// Find an image with a random name. Make sure it fails.
func TestFindImageDoesNotFindNonExist(t *testing.T) {
	initDockerClientOrSkip(t)

	_, ow := rpctest.NewRecordedOutputWriter(t.Name())
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	imageName := strconv.Itoa(rand.Int())

	_, found, err := docker.FindImage(ctx, ow, cli, imageName)
	require.NoError(t, err)
	require.False(t, found)
}

// Create a new Dockerfile with fresh content.
// Use BuildImage to build it. Make sure it exists.
func TestBuildImageBuildsImages(t *testing.T) {
	initDockerClientOrSkip(t)

	_, ow := rpctest.NewRecordedOutputWriter(t.Name())
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	rndname, dir := randomBuildContext(t)
	defer os.Remove(dir) // cleanup

	// Build the image
	opts := docker.BuildImageOpts{
		Name:     rndname,
		BuildCtx: dir,
	}
	err := docker.BuildImage(ctx, ow, cli, &opts)
	require.NoError(t, err)

	// Check that it exists.
	_, found, err := docker.FindImage(ctx, ow, cli, rndname)
	require.NoError(t, err)
	require.True(t, found)
}

// Ensure a container exists. and then make sure CheckContainer can find it.
func TestCheckContainerFindsExistingContainer(t *testing.T) {
	initDockerClientOrSkip(t)

	_, ow := rpctest.NewRecordedOutputWriter(t.Name())
	ctx := context.Background()

	image := "hello-world:latest"
	id, name := pullCreateDelete(t, ctx, image)
	cont, err := docker.CheckContainer(ctx, ow, cli, name)
	require.NoError(t, err)
	require.NotNil(t, cont)
	require.Equal(t, id, cont.ID)
}

// Try to find a container which does not exist. Make sure it cant be found.
func TestCheckContainerDoesNotFindNonExist(t *testing.T) {
	initDockerClientOrSkip(t)

	_, ow := rpctest.NewRecordedOutputWriter(t.Name())
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	name := strconv.Itoa(rand.Int())

	cont, err := docker.CheckContainer(ctx, ow, cli, name)
	require.NoError(t, err)
	require.Nil(t, cont)
}

func initDockerClientOrSkip(t *testing.T) {
	t.Helper()
	var err error
	cli, err = client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}
	cli.NegotiateAPIVersion(context.Background()) // fails silently
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err = cli.Ping(ctx); err != nil {
		t.Skipf("docker daemon not available; err: %s", err)
	}
}

// Utility functions for test.
// These functions intentionally *don't* use tested functions.

// pull an image from docker hub.
func pullImage(ctx context.Context, imageID string) error {
	options := types.ImagePullOptions{}
	c, err := cli.ImagePull(ctx, imageID, options)
	if err != nil {
		return err
	}
	_, err = ioutil.ReadAll(c)
	return err
}

func randomBuildContext(t *testing.T) (name string, dir string) {
	rndname := strings.ToLower(t.Name() + "-" + strconv.Itoa(rand.Int()))

	d, err := ioutil.TempDir("", rndname)
	require.NoError(t, err)

	// Create a simple Dockerfile.
	dockerfile := filepath.Join(d, "Dockerfile")
	cont := fmt.Sprintf("FROM scratch\nCOPY Dockerfile /\n# random comment %s\n", rndname)
	err = ioutil.WriteFile(dockerfile, []byte(cont), os.ModePerm)
	require.NoError(t, err)
	return rndname, d
}

// cleanup function which deletes a container
func deleteContainerFn(ctx context.Context, t *testing.T, containerID string) func() {
	options := types.ContainerRemoveOptions{
		RemoveVolumes: true,
		RemoveLinks:   false,
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
	var (
		containerCfg = container.Config{Image: imageID}
		hostCfg      = container.HostConfig{}
		networkCfg   = network.NetworkingConfig{}
	)

	body, err := cli.ContainerCreate(ctx, &containerCfg, &hostCfg, &networkCfg, containerName)
	if err != nil {
		return "", err
	}
	return body.ID, err
}

// pull an image from docker hub.
// create a container with a randomized name
// Configure the container to be deleted when the test completes.
// return the container ID.
func pullCreateDelete(t *testing.T, ctx context.Context, imageName string) (containerID string, containerName string) {
	containerName = t.Name() + "-" + strconv.FormatUint(rand.Uint64(), 16)

	err := pullImage(ctx, imageName)
	require.NoError(t, err)

	containerID, err = createContainer(ctx, containerName, imageName)
	require.NoError(t, err)

	t.Cleanup(deleteContainerFn(ctx, t, containerID))
	return containerID, containerName
}
