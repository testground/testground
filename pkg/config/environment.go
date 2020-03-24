package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"

	"github.com/BurntSushi/toml"
)

const (
	EnvTestgroundSrcDir  = "TESTGROUND_SRCDIR"
	EnvTestgroundWorkDir = "TESTGROUND_WORKDIR"

	DefaultListenAddr = "localhost:8042"
)

const gomodHeader = "module github.com/ipfs/testground"

var (
	ErrUnknownSrcDir = errors.New("unable to determine testground src dir")
)

// EnvConfig represents an environment configuration read
type EnvConfig struct {
	AWS             AWSConfig            `toml:"aws"`
	DockerHub       DockerHubConfig      `toml:"dockerhub"`
	BuildStrategies map[string]ConfigMap `toml:"build_strategies"`
	RunStrategies   map[string]ConfigMap `toml:"run_strategies"`
	Daemon          DaemonConfig         `toml:"daemon"`
	Client          ClientConfig         `toml:"client"`
	SrcDir          string
	WrkDir          string
}

func (e EnvConfig) SourceDir() string {
	return e.SrcDir
}

func (e EnvConfig) WorkDir() string {
	return e.WrkDir
}

type AWSConfig struct {
	AccessKeyID     string `toml:"access_key_id"`
	SecretAccessKey string `toml:"secret_access_key"`
	Region          string `toml:"region"`
}

type DockerHubConfig struct {
	Repo        string `toml:"repo"`
	Username    string `toml:"username"`
	AccessToken string `toml:"access_token"`
}

type DaemonConfig struct {
	Listen string `toml:"listen"`
}

type ClientConfig struct {
	Endpoint string `toml:"endpoint"`
}

type ConfigMap map[string]interface{}

func GetEnvConfig() (*EnvConfig, error) {
	ec := &EnvConfig{}

	srcdir, _ := locateSrcDir()
	//if err != nil {
	//return nil, err
	//}

	workdir, err := locateWorkDir()
	if err != nil {
		return nil, err
	}

	ec.SrcDir = srcdir
	ec.WrkDir = workdir

	// try to load the .env.toml file.
	// .env.toml is not required
	_, _ = toml.DecodeFile(filepath.Join(srcdir, ".env.toml"), ec)

	applyDefaults(ec)

	return ec, nil
}

func applyDefaults(ec *EnvConfig) {
	if ec.Daemon.Listen == "" {
		ec.Daemon.Listen = DefaultListenAddr
	}
	if ec.Client.Endpoint == "" {
		ec.Client.Endpoint = DefaultListenAddr
	}
}

// locateSrcDir attempts to locate the source directory for the testground. We
// need to know this directory in order to build test plans.
func locateSrcDir() (string, error) {
	// 1. If the env variable is set, we use its value, checking if it points to
	// the repo.
	if v, ok := os.LookupEnv(EnvTestgroundSrcDir); ok && isTestgroundRepo(v) {
		return v, nil
	}

	// 2. Try the executable directory.
	// 3. Try the working directory.
	for _, dirFn := range []func() (string, error){os.Executable, os.Getwd} {
		path, err := dirFn()
		if err != nil {
			return "", err
		}

		for isNotRootDir(path) {
			if isTestgroundRepo(path) {
				os.Setenv(EnvTestgroundSrcDir, path)
				return path, nil
			}
			path = filepath.Dir(path)
		}
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

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
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

// isNotRootDir checks if a certain path is a root or not. For Unix-like
// systems, it just checks it's longer than one character (usually "/").
// On Windows, we need to check if it's not a drive, such as "C:/".
func isNotRootDir(path string) bool {
	if runtime.GOOS != "windows" {
		return len(path) > 1
	}

	return filepath.VolumeName(path) != path[:len(path)-1]
}
