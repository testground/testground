//go:build integration
// +build integration

package utils

import (
	"io/ioutil"
	"os"
	"testing"
)

// Create a temporary directory, and move there.
//
// Returns the path to the temporary directory, and a cleanup function that
// restore the working directory and remove temporary dirs.
func fromTemporaryDirectory(t *testing.T) (err error, dir string, cleanup func()) {
	t.Helper()
	cleanTemporaryDirectory, moveBackToWorkingDirectory := func(){}, func(){}
	cleanup = func() {
			moveBackToWorkingDirectory()
			cleanTemporaryDirectory()
	}

	// If something goes wrong at any point of the setup, cleanup.
	defer func() {
		if (err != nil) {
			cleanup()
		}
	}()

	// Create a temporary directory.
	dir, err = ioutil.TempDir("", "testground")
	if err != nil {
		t.Fatal(err)
	}
	cleanTemporaryDirectory = func() {
		if err = os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
	}

	// Then change directory
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	moveBackToWorkingDirectory = func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	}

	if err = os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	return nil, dir, cleanup
}
