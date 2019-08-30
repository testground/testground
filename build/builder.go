package build

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"

	"github.com/ipfs/testground/logging"
)

const (
	// EnvBaseDir is the env variable that may contain the base directory for the testground installation.
	EnvBaseDir = "TESTGROUND_BASEDIR"

	gomodHeader = "module github.com/ipfs/testground"
)

var (
	ErrUnknownBaseDir = errors.New("unable to determine base dir of testground source")
)

// LocateBaseDir returns the base directory of the test ground. It is used to find the test plan subdirectories.
func LocateBaseDir() (string, error) {
	// 1. If the env variable is set, we use its value, checking if it points to the repo.
	if v, ok := os.LookupEnv(EnvBaseDir); ok && isRepo(v) {
		logging.S().Infow("resolved basedir from env variable", "basedir", v)
		return v, nil
	}

	// 2. Fallback to the working directory.
	path, err := os.Executable()
	if err != nil {
		return "", ErrUnknownBaseDir
	}

	for len(path) > 1 {
		logging.S().Debugf("attempting to resolve basedir", "trying", path)
		if path = filepath.Dir(path); isRepo(path) {
			logging.S().Infow("resolved basedir by guessing", "basedir", path)
			return path, nil
		}
	}

	logging.S().Errorw("unable to resolve basedir")
	return "", ErrUnknownBaseDir
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
