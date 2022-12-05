//go:build integration
// +build integration

package utils

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"testing"
)

func Setup(t *testing.T) {
	t.Helper()

	err := runImport(t, "../../plans", "testground")
	if err != nil {
		t.Fatal(err)
	}
}

func RunSingle(t *testing.T, params RunSingleParams) (*RunResult, error) {
	t.Helper()

	// Create a temporary directory for the test.
	err, dir, cleanup := fromTemporaryDirectory(t)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	// Start the daemon
	srv := setupDaemon(t)
	defer func() {
		err := runTerminate(t, srv, params.Runner)
		srv.Shutdown(context.Background()) //nolint

		if err != nil {
			t.Fatal(err)
		}
	}()

	err = runHealthcheck(t, srv, params.Runner)
	if err != nil {
		t.Fatal(err)
	}

	// Run the test.
	result, err := runSingle(t, params, srv)
	if err != nil && result == nil {
		t.Fatal(err)
	}

	// Collect the results.
	if params.Collect {
		collectedPath := filepath.Join(dir, "collected.tgz")
		collectedDestination, err := ioutil.TempDir("", "testground-collected")
		if err != nil {
			t.Fatal(err)
		}
		// TODO: cleanup that folder in case of errors.

		err = ExtractTarGz(collectedPath, collectedDestination)
		if err != nil {
			t.Fatal(err)
		}

		// Get the one file path in that folder (which is the run-id folder)
		files, err := ioutil.ReadDir(collectedDestination)
		if err != nil {
			t.Fatal(err)
		}

		// Check that there is only one directory in the folder
		if len(files) != 1 || !files[0].IsDir() {
			t.Fatal("expected one directory in the collected folder")
		}

		collectFolder := filepath.Join(collectedDestination, files[0].Name())
		result.CollectFolder = collectFolder
		result.Cleanup = func() {
			if err := os.RemoveAll(collectedDestination); err != nil {
				t.Fatal(err)
			}
		}
	}

	return result, err
}

func RunComposition(t *testing.T, params RunCompositionParams) (*RunResult, error) {
	t.Helper()

	// Rewrite path before changing directories.
	params = params.withAbsolutePath()

	// Create a temporary directory for the test.
	err, _, cleanup := fromTemporaryDirectory(t)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	// Start the daemon
	srv := setupDaemon(t)
	defer func() {
		err := runTerminate(t, srv, params.Runner)
		srv.Shutdown(context.Background()) //nolint

		if err != nil {
			t.Fatal(err)
		}
	}()

	err = runHealthcheck(t, srv, params.Runner)
	if err != nil {
		t.Fatal(err)
	}

	// Run the test.
	result, err := runComposition(t, params, srv)
	if err != nil && result == nil {
		t.Fatal(err)
	}

	// Collect the results.
	// if params.collect {
	// 	result, err = Collect(t, dir, result)
	// 	require.NoError(t, err)
	// }

	return result, err
}
