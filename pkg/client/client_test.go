package client

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	withoutIgnoreFileDir = "fixtures/without_ignore"
	withIgnoreFileDir    = "fixtures/with_ignore"
)

func TestFilteredDirectoryWithoutIgnore(t *testing.T) {
	dir, err := getFilteredDirectory(withoutIgnoreFileDir)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = os.RemoveAll(dir)
	}()

	require.NotEqual(t, withoutIgnoreFileDir, dir)
}

func TestFilteredDirectoryWithIgnore(t *testing.T) {
	dir, err := getFilteredDirectory(withIgnoreFileDir)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = os.RemoveAll(dir)
	}()

	require.NotEqual(t, withoutIgnoreFileDir, dir)

	mustExist := []string{
		"b/file.txt",
		"d/file.txt",
	}

	mustNotExist := []string{
		"a",
		"b/c",
		"b/d",
		"b/file.out",
	}

	for _, file := range mustExist {
		require.FileExists(t, filepath.Join(dir, file))
	}

	for _, file := range mustNotExist {
		require.NoFileExists(t, filepath.Join(dir, file))
	}
}
