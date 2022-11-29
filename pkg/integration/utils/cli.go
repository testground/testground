//go:build integration
// +build integration

package utils

import (
	"log"
	"os/exec"

	"testing"
)

// use the CLI and call the command `docker pull` for each image in a list of images
func DockerPull(t *testing.T, images ...string) {
	for _, image := range images {
		t.Logf("$ docker pull %s", image)
		cmd := exec.Command("docker", "pull", image)
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
	}
}
