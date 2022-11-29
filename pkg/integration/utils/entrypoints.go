//go:build integration
// +build integration

package utils

import (
	"context"
	"io/ioutil"
	"os"

	"testing"
)


func Setup(t *testing.T) {
	t.Helper()

	err := runImport(t, "../../plans", "testground")
	if err != nil {
		t.Fatal(err)
	}
}

func Run(t *testing.T, params RunSingle) (*RunResult, error) {
	t.Helper()

	// Create a temporary directory for the test.
	dir, err := ioutil.TempDir("", "testground")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
	}()

	// Change directory during the test
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	}()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

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
	// if params.collect {
	// 	result, err = Collect(t, dir, result)
	// 	require.NoError(t, err)
	// }

	return result, err
}

func RunAComposition(t *testing.T, params RunComposition) (*RunResult, error) {
	t.Helper()

	// Rewrite path before changing directories.
	params = params.withAbsolutePath()

	// Create a temporary directory for the test.
	dir, err := ioutil.TempDir("", "testground")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
	}()

	// Change directory during the test
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	}()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

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
