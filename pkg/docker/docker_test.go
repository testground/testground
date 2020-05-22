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

	"github.com/testground/testground/pkg/docker"
	"github.com/testground/testground/pkg/rpc/rpctest"
)

var (
	cli *client.Client
)

func init() {
	var err error
	cli, err = client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}

	cli.NegotiateAPIVersion(context.Background())
	rand.Seed(time.Now().UnixNano())
}

func errfail(t *testing.T, err error) {
	if err != nil {
		t.Error(err)
		t.FailNow()
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

func randomBuildContext(t *testing.T) (string, string) {
	rndname := strings.ToLower(t.Name() + "-" + strconv.Itoa(rand.Int()))

	d, err := ioutil.TempDir("", rndname)
	errfail(t, err)

	// Create a simple Dockerfile.
	dockerfile := filepath.Join(d, "Dockerfile")
	cont := fmt.Sprintf("FROM scratch\nCOPY Dockerfile /\n# random comment %s\n", rndname)
	err = ioutil.WriteFile(dockerfile, []byte(cont), os.ModePerm)
	errfail(t, err)
	return rndname, d
}

// cleanup function which deletes a container
func deleteContainer(ctx context.Context, t *testing.T, containerID string) func() {
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
func pullCreateDelete(t *testing.T, ctx context.Context, imageName string) (containerID string, containerName string) {
	containerName = t.Name() + "-" + strconv.FormatUint(rand.Uint64(), 16)

	err := pullImage(ctx, imageName)
	errfail(t, err)

	containerID, err = createContainer(ctx, containerName, imageName)
	errfail(t, err)

	t.Cleanup(deleteContainer(ctx, t, containerID))
	return
}

// Pull an image (to ensure it exists) then make sure FindImage can find it.
func TestFindImageFindsImages(t *testing.T) {
	_, ow := rpctest.NewRecordedOutputWriter(t.Name())
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	imageName := "hello-world"
	err := pullImage(ctx, imageName)
	errfail(t, err)

	_, found, err := docker.FindImage(ctx, ow, cli, imageName)
	errfail(t, err)

	if !found {
		t.Fail()
	}
}

// Find an image with a random name. Make sure it fails.
func TestFindImageDoesNotFindNonExist(t *testing.T) {
	_, ow := rpctest.NewRecordedOutputWriter(t.Name())
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	imageName := strconv.Itoa(rand.Int())

	_, found, err := docker.FindImage(ctx, ow, cli, imageName)
	errfail(t, err)

	if found {
		t.Fail()
	}
}

// Create a new Dockerfile with fresh content.
// Use BuildImage to build it. Make sure it exists.
func TestBuildImageBuildsImages(t *testing.T) {
	_, ow := rpctest.NewRecordedOutputWriter(t.Name())
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	rndname, d := randomBuildContext(t)

	// Build the image
	opts := docker.BuildImageOpts{
		Name:     rndname,
		BuildCtx: d,
	}
	err := docker.BuildImage(ctx, ow, cli, &opts)
	errfail(t, err)

	// Check that it exists.
	_, found, err := docker.FindImage(ctx, ow, cli, rndname)
	errfail(t, err)
	if !found {
		t.Fail()
	}
}

// Ensure a container exists. and then make sure CheckContainer can find it.
func TestCheckContainerFindsExistingContainer(t *testing.T) {
	_, ow := rpctest.NewRecordedOutputWriter(t.Name())
	ctx := context.Background()

	image := "hello-world:latest"
	id, name := pull_create_delete(ctx, t, image)
	cont, err := docker.CheckContainer(ctx, ow, cli, name)
	errfail(t, err)
	if cont == nil {
		t.Log("container not found. nil.")
		t.Fail()
	}
	if cont.ID != id {
		t.Fatalf("incorrect container found. id does not match that created. expected %s, got %s", id, cont.ID)
	}
}

// Try to find a container which does not exist. Make sure it cant be found.
func TestCheckContainerDoesNotFindNonExist(t *testing.T) {
	_, ow := rpctest.NewRecordedOutputWriter(t.Name())
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	name := strconv.Itoa(rand.Int())

	cont, err := docker.CheckContainer(ctx, ow, cli, name)
	errfail(t, err)

	if cont != nil {
		t.Fail()
	}
}
