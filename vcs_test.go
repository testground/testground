package tpipeline

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckoutCommit(t *testing.T) {
	dir, err := CreateTempDir()
	if err != nil {
		t.Fatal(err)
	}

	err = CheckoutCommit("7a185489a5a7562c7ac710ae3b174628c97ae807", dir)
	if err != nil {
		t.Fatal(err)
	}

	bytes, err := ioutil.ReadFile(filepath.Join(dir, "version.go"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(bytes), "0.4.21-rc3") {
		t.Error("local checkout doesn't point to specified commit")
	}
}