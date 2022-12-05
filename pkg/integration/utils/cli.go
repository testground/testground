//go:build integration
// +build integration

package utils

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"

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

func ExtractTarGz(src, dst string) error {
	targzStream, err := os.Open(src)
	if err != nil {
		return err
	}
	defer targzStream.Close()

	tarStream, err := gzip.NewReader(targzStream)
	if err != nil {
		return err
	}

	tarReader := tar.NewReader(tarStream)
	for true {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("ExtractTarGz: Next() failed: %w", err)
		}

		target := filepath.Join(dst, header.Name)

		switch header.Typeflag {

		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return fmt.Errorf("ExtractTarGz: Mkdir() failed: %w", err)
				}
			}
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("ExtractTarGz: Create() failed: %w", err)
			}

			if _, err := io.Copy(f, tarReader); err != nil {
				return fmt.Errorf("ExtractTarGz: Copy() failed: %w", err)
			}

			f.Close()
		default:
			// TODO: clean up behind us
			return fmt.Errorf("ExtractTarGz: unsupported: %v in %s", header.Typeflag, header.Name)
		}
	}

	return nil
}
