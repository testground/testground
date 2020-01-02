package utils

import (
	"fmt"
	"os"

	"github.com/dustin/go-humanize"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/testground/sdk/runtime"
)

// TestConfig is a test configuration.
type TestConfig []*TestDir

// TestDir is a test directory or file. If the depth is 0 (zero), then this it
// is a file.
type TestDir struct {
	Path  string
	Depth uint
	Size  int64
}

// GetTestConfig retrieves the configuration from the runtime environment.
func GetTestConfig(runenv *runtime.RunEnv, acceptFiles bool, acceptDirs bool) (cfg TestConfig, err error) {
	cfg = TestConfig{}

	if acceptFiles {
		sizes := runenv.SizeArrayParam("file-sizes")
		for _, size := range sizes {
			file, err := runenv.CreateRandomFile(os.TempDir(), int64(size))
			if err != nil {
				return nil, err
			}

			runenv.Message("%s: %s file created", file, humanize.Bytes(size))

			cfg = append(cfg, &TestDir{
				Path: file,
				Size: int64(size),
			})
		}
	}

	if acceptDirs {
		dirConfigs := []rawDirConfig{}
		runenv.JSONParam("dir-cfg", &dirConfigs)
		for _, dir := range dirConfigs {
			n, err := humanize.ParseBytes(dir.Size)
			if err != nil {
				return nil, err
			}

			path, err := runenv.CreateRandomDirectory(os.TempDir(), dir.Depth)
			if err != nil {
				return nil, err
			}

			_, err = runenv.CreateRandomFile(path, int64(n))
			if err != nil {
				return nil, err
			}

			runenv.Message("%s: %s directory created", humanize.Bytes(n), path)

			cfg = append(cfg, &TestDir{
				Path:  path,
				Depth: dir.Depth,
				Size:  int64(n),
			})
		}
	}

	return cfg, nil
}

type OsTestFunction func(path string, size int64, isDir bool) (string, error)

func (tc TestConfig) ForEachPath(runenv *runtime.RunEnv, fn OsTestFunction) error {
	for _, cfg := range tc {
		err := func() error {
			var cid string
			var err error

			if cfg.Depth == 0 {
				cid, err = fn(cfg.Path, cfg.Size, false)
			} else {
				cid, err = fn(cfg.Path, cfg.Size, true)
			}

			if err != nil {
				return fmt.Errorf("%s: failed to add %s", cfg.Path, err)
			}

			runenv.Message("%s: %s added", cfg.Path, cid)
			return nil
		}()

		if err != nil {
			return err
		}
	}

	return nil
}

func ConvertToUnixfs(path string, isDir bool) (files.Node, error) {
	var unixfsFile files.Node
	var err error

	if isDir {
		unixfsFile, err = GetPathToUnixfsDirectory(path)
	} else {
		unixfsFile, err = GetPathToUnixfsFile(path)
	}

	if err != nil {
		return unixfsFile, fmt.Errorf("failed to get Unixfs file from path: %s", err)
	}

	return unixfsFile, err
}

func (tc TestConfig) Cleanup() {
	for _, dir := range tc {
		os.RemoveAll(dir.Path)
	}
}

type testConfig struct {
	Depth uint
	Size  int64
}

type rawDirConfig struct {
	Depth uint   `json:"depth"`
	Size  string `json:"size"`
}
