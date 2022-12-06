package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/testground/testground/pkg/logging"
)

const (
	EnvTestgroundHomeDir = "TESTGROUND_HOME"

	// DefaultListenAddr is a host:port value, where we set up an HTTP endpoint.
	// In the future we will support an HTTPS mode.
	DefaultListenAddr = "localhost:8042"

	// DefaultClientURL is the HTTP(S) endpoint of the server.
	DefaultClientURL = "http://" + DefaultListenAddr

	DefaultInfluxDBEndpoint = "http://localhost:8086"

	DefaultTaskRepoType = "memory"

	DefaultWorkers = 2

	DefaultQueueSize = 100
)

func (e *EnvConfig) Load() error {
	err := e.EnsureMinimalConfig()
	if err != nil {
		return err
	}

	// parse the .env.toml file, if it exists.
	f := filepath.Join(e.dirs.Home(), ".env.toml")
	if _, err := os.Stat(f); err == nil {
		// try to load the optional .env.toml file
		_, err = toml.DecodeFile(f, e)
		if err != nil {
			return fmt.Errorf("found .env.toml at %s, but failed to parse: %w", f, err)
		}
		logging.S().Infof(".env.toml loaded from: %s", f)
	} else {
		logging.S().Infof("no .env.toml found at %s; running with defaults", f)
	}
	return nil
}

func (e *EnvConfig) EnsureMinimalConfig() error {
	// apply fallbacks.
	e.Daemon.Listen = defaultString(e.Daemon.Listen, DefaultListenAddr)
	e.Daemon.InfluxDBEndpoint = defaultString(e.Daemon.InfluxDBEndpoint, DefaultInfluxDBEndpoint)
	e.Client.Endpoint = defaultString(e.Client.Endpoint, DefaultClientURL)
	e.Daemon.Scheduler.Workers = defaultInt(e.Daemon.Scheduler.Workers, DefaultWorkers)
	e.Daemon.Scheduler.QueueSize = defaultInt(e.Daemon.Scheduler.QueueSize, DefaultQueueSize)
	e.Daemon.Scheduler.TaskRepoType = defaultString(e.Daemon.Scheduler.TaskRepoType, DefaultTaskRepoType)

	// calculate home directory; use env var, or fall back to $HOME/testground
	// otherwise.
	var home string
	if v, ok := os.LookupEnv(EnvTestgroundHomeDir); ok {
		// we have an env var.
		home = v
	} else {
		// fallback to $HOME/testground.
		v, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to obtain user home dir: %w", err)
		}
		home = filepath.Join(v, "testground")
	}

	switch fi, err := os.Stat(home); {
	case os.IsNotExist(err):
		logging.S().Infof("creating home directory at %s", home)
		if err := os.MkdirAll(home, 0777); err != nil {
			return fmt.Errorf("failed to create home directory at %s: %w", home, err)
		}
	case err == nil:
		logging.S().Infof("using home directory: %s", home)
	case !fi.IsDir():
		return fmt.Errorf("home path is not a directory %s", home)
	}

	// ensure home and children directories exist.
	e.dirs = Directories{home}
	for _, d := range []string{
		e.dirs.Home(),
		e.dirs.Outputs(),
		e.dirs.Plans(),
		e.dirs.SDKs(),
		e.dirs.Work(),
		e.dirs.Daemon(),
	} {
		if err := ensureDir(d); err != nil {
			return fmt.Errorf("failed to check/create directory %s: %w", d, err)
		}
	}

	return nil
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

func defaultString(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
func defaultInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}