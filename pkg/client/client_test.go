package client

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	withoutIgnoreFileDir = "fixtures/without_ignore"
	withIgnoreFileDir    = "fixtures/with_ignore"
)

func TestFilteredDirectoryWithoutIgnore(t *testing.T) {
	dir, tmp, err := getFilteredDirectory(withoutIgnoreFileDir)
	if err != nil {
		t.Fatal(err)
	}

	if tmp {
		t.Fatal("temp bool must be false")
	}

	if dir != withoutIgnoreFileDir {
		t.Fatalf("returned directory must be %s, received %s", withoutIgnoreFileDir, dir)
	}
}

func TestFilteredDirectoryWithIgnore(t *testing.T) {
	dir, tmp, err := getFilteredDirectory(withIgnoreFileDir)
	if err != nil {
		t.Fatal(err)
	}

	if !tmp {
		t.Fatal("temp bool must be true")
	}

	defer func() {
		_ = os.RemoveAll(dir)
	}()

	mustExist := []string{
		"b/file.txt",
	}

	mustNotExist := []string{
		"a",
		"b/c",
		"b/file.out",
	}

	for _, file := range mustExist {
		if _, err := os.Stat(filepath.Join(dir, file)); err != nil {
			t.Fatalf("file must exist; %s", err)
		}
	}

	for _, file := range mustNotExist {
		if _, err := os.Stat(filepath.Join(dir, file)); !os.IsNotExist(err) {
			t.Fatalf("file must not exist; %s", err)
		}
	}

	t.Log(dir)
}
