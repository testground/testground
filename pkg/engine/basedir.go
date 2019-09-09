package engine

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	EnvTestgroundDir = "TESTGROUND_BASEDIR"
	gomodHeader      = "module github.com/ipfs/testground"
)

var (
	BaseDir string

	ErrUnknownPlanDir = errors.New("unable to determine base dir of testground source")
)

func init() {
	// 1. If the env variable is set, we use its value, checking if it points to the repo.
	if v, ok := os.LookupEnv(EnvTestgroundDir); ok && isRepo(v) {
		BaseDir = v
		fmt.Printf("resolved testground base dir from env variable: %s\n", BaseDir)
		return
	}

	// 2. Fallback to the working directory.
	path, err := os.Executable()
	if err != nil {
		panic(ErrUnknownPlanDir)
	}

	for len(path) > 1 {
		fmt.Printf("attempting to guess testground base directory; for better control set ${%s}\n", EnvTestgroundDir)
		if path = filepath.Dir(path); isRepo(path) {
			BaseDir = path
			os.Setenv(EnvTestgroundDir, path)
			fmt.Printf("successfully located testground base directory: %s\n", path)
			return
		}
	}
	panic(ErrUnknownPlanDir)
}

func isRepo(dir string) bool {
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		return false
	}
	f, err := os.Open(filepath.Join(dir, "go.mod"))
	if err != nil {
		return false
	}
	s := bufio.NewScanner(f)
	if !s.Scan() {
		return false
	}
	return s.Text() == gomodHeader
}
