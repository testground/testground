package engine

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func removeEnvTestgroundSrcDir(tb testing.TB) func() {
	if v, ok := os.LookupEnv(EnvTestgroundSrcDir); ok {
		os.Unsetenv(EnvTestgroundSrcDir)
		return func() { os.Setenv(EnvTestgroundSrcDir, v) }
	}
	// no-op
	return func() {}
}

func setupFakeTestgroundRepo(tb testing.TB) (string, func()) {
	content := []byte(`module github.com/ipfs/testground

	go 1.13
	`)
	srcdir, err := ioutil.TempDir("", "testground")
	if err != nil {
		tb.Fatal(err)
	}

	tmpmod := filepath.Join(srcdir, "go.mod")
	if err := ioutil.WriteFile(tmpmod, content, 0666); err != nil {
		tb.Fatal(err)
	}

	return srcdir, func() { os.RemoveAll(srcdir) }
}

func Test_isTestgroundRepo(t *testing.T) {
	srcdir, cleanup := setupFakeTestgroundRepo(t)
	defer cleanup()

	if isRepo := isTestgroundRepo(srcdir); !isRepo {
		t.Fail()
	}
}

func Test_locateSrcDir(t *testing.T) {
	cleanEnv := removeEnvTestgroundSrcDir(t)
	defer cleanEnv()

	srcdir, cleanRepo := setupFakeTestgroundRepo(t)
	defer cleanRepo()

	err := os.Chdir(srcdir)
	if err != nil {
		t.Fatal(err)
	}

	got, err := locateSrcDir()
	if err != nil {
		t.Error(err)
		t.Fail()
	}

	if got != srcdir {
		t.Fail()
	}
}
