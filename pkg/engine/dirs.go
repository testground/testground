package engine

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
)

const (
	EnvTestgroundSrcDir  = "TESTGROUND_SRCDIR"
	EnvTestgroundWorkDir = "TESTGROUND_WORKDIR"
)

const gomodHeader = "module github.com/ipfs/testground"

var (
	ErrUnknownSrcDir = errors.New("unable to determine testground src dir")
)

// SourceDir is an accessor returning the source directory of this engine.
func (e *Engine) SourceDir() string {
	return e.dirs.src
}

// WorkDir is an accessor returning the work directory of this engine.
func (e *Engine) WorkDir() string {
	return e.dirs.work
}

// locateSrcDir attempts to locate the source directory for the testground.
func locateSrcDir() (string, error) {
	// 1. If the env variable is set, we use its value, checking if it points to the repo.
	if v, ok := os.LookupEnv(EnvTestgroundSrcDir); ok && isTestgroundRepo(v) {
		fmt.Printf("resolved testground base dir from env variable: %s\n", v)
		return v, nil
	}

	// 2. Fallback to the working directory.
	path, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for len(path) > 1 {
		fmt.Printf("attempting to guess testground base directory; for better control set ${%s}\n", EnvTestgroundSrcDir)
		if isTestgroundRepo(path) {
			os.Setenv(EnvTestgroundSrcDir, path)
			fmt.Printf("successfully located testground base directory: %s\n", path)
			return path, nil
		}
		path = filepath.Dir(path)
	}

	return "", ErrUnknownSrcDir
}

// locateWorkDir attempts to locate the work directory.
func locateWorkDir() (string, error) {
	// 1. Use work directory if explicitly passed in as an env var.
	if v, ok := os.LookupEnv(EnvTestgroundWorkDir); ok {
		return v, ensureDir(v)
	}

	// 2. Use "$HOME/.testground" as the work directory.
	home, ok := os.LookupEnv("HOME")
	if !ok {
		return "", errors.New("$HOME env variable not declared; cannot calculate work directory")
	}
	p := path.Join(home, ".testground")
	return p, ensureDir(p)
}

// isTestgroundRepo verifies if the specified path contains the testground
// source repo.
func isTestgroundRepo(path string) bool {
	fi, err := os.Stat(path)
	if err != nil || !fi.IsDir() {
		return false
	}
	f, err := os.Open(filepath.Join(path, "go.mod"))
	if err != nil {
		return false
	}
	s := bufio.NewScanner(f)
	if !s.Scan() {
		return false
	}
	return s.Text() == gomodHeader
}

// ensureDir checks whether the specified path is a directory, and if not it
// attempts to create it.
func ensureDir(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		// We need to create the directory.
		return os.MkdirAll(path, os.ModePerm)
	}

	if !fi.IsDir() {
		return fmt.Errorf("path %s exists, and it is not a directory", path)
	}
	return nil
}
