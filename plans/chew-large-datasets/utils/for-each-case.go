package utils

import (
	"fmt"
	"os"

	"github.com/dustin/go-humanize"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/testground/sdk/runtime"
)

type testConfig struct {
	Depth uint
	Size  int64
}

type rawDirConfig struct {
	Depth uint   `json:"depth"`
	Size  string `json:"size"`
}

func ForEachCase(runenv *runtime.RunEnv, fn func(files.Node, bool) error) error {
	a, err := getAddTestsConfig(runenv)
	if err != err {
		return fmt.Errorf("could not parse test parameters: %s", err)
	}

	for _, cfg := range a {
		err := func() error {
			path, err := CreateRandomDirectory(runenv, os.TempDir(), cfg.Depth)
			if err != nil {
				return err
			}

			if cfg.Depth != 0 {
				defer os.RemoveAll(path)
			}

			file, err := CreateRandomFile(runenv, path, cfg.Size)
			if err != nil {
				return err
			}

			if cfg.Depth == 0 {
				defer os.Remove(file.Name())
			}

			var unixfsFile files.Node

			if cfg.Depth == 0 {
				unixfsFile, err = GetPathToUnixfsFile(file.Name())
			} else {
				unixfsFile, err = GetPathToUnixfsDirectory(path)
			}

			if err != nil {
				return fmt.Errorf("failed to get Unixfs file from path: %s", err)
			}

			err = fn(unixfsFile, cfg.Depth != 0)
			if err != nil {
				return err
			}

			return nil
		}()

		if err != nil {
			return err
		}
	}

	return nil
}

func getAddTestsConfig(runenv *runtime.RunEnv) (tests []testConfig, err error) {
	// --test-param file-sizes='["10GB"]'
	sizes := runenv.StringArrayParamD("file-sizes", []string{"1MB", "1GB", "10GB"})

	for _, size := range sizes {
		n, err := humanize.ParseBytes(size)
		if err != nil {
			return tests, err
		}
		tests = append(tests, testConfig{
			Size:  int64(n),
			Depth: 0,
		})
	}

	// --test-param dir-cfg='[{"depth": 10, "size": "1MB"}, {"depth": 100, "size": "1MB"}]
	dirConfigs := []rawDirConfig{}
	_ = runenv.JSONParam("dir-cfg", &dirConfigs)

	for _, cfg := range dirConfigs {
		n, err := humanize.ParseBytes(cfg.Size)
		if err != nil {
			return tests, err
		}

		tests = append(tests, testConfig{
			Depth: cfg.Depth,
			Size:  int64(n),
		})
	}

	return tests, nil
}
