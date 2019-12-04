package utils

import (
	"fmt"
	"os"

	"github.com/dustin/go-humanize"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/testground/sdk/runtime"
)

type DirectoryConfig struct {
	Depth uint
	Size  int64
}

type AddTestsConfig struct {
	Sizes       []int64
	Directories []DirectoryConfig
}

type dirConfig struct {
	Depth uint   `json:"depth"`
	Size  string `json:"size"`
}

func (a *AddTestsConfig) ForEachSize(runenv *runtime.RunEnv, fn func(files.File) error) error {
	for _, size := range a.Sizes {
		err := func() error {
			file, err := CreateRandomFile(runenv, os.TempDir(), size)
			if err != nil {
				return err
			}
			defer os.Remove(file.Name())

			unixfsFile, err := GetPathToUnixfsFile(file.Name())
			if err != nil {
				return fmt.Errorf("failed to get Unixfs file from path: %s", err)
			}

			err = fn(unixfsFile)
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

func GetAddTestsConfig(runenv *runtime.RunEnv) (tests AddTestsConfig, err error) {
	// --test-param file-sizes='["10GB"]'
	sizes := runenv.StringArrayParamD("file-sizes", []string{"1MB", "1GB", "10GB"})

	for _, size := range sizes {
		n, err := humanize.ParseBytes(size)
		if err != nil {
			return tests, err
		}
		tests.Sizes = append(tests.Sizes, int64(n))
	}

	// --test-param dir-cfg='[{"depth": 10, "size": "1MB"}, {"depth": 100, "size": "1MB"}]
	dirConfigs := []dirConfig{}
	_ = runenv.JSONParam("dir-cfg", &dirConfigs)

	for _, cfg := range dirConfigs {
		n, err := humanize.ParseBytes(cfg.Size)
		if err != nil {
			return tests, err
		}

		tests.Directories = append(tests.Directories, DirectoryConfig{
			Depth: cfg.Depth,
			Size:  int64(n),
		})
	}

	fmt.Println(tests)
	return tests, nil
}
