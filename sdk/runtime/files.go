package runtime

import (
	"bufio"
	"crypto/rand"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// CreateRandomFile creates a file of the specified size (in bytes) within the
// specified directory path and returns its path.
func (re *RunEnv) CreateRandomFile(directoryPath string, size int64) (string, error) {
	file, err := ioutil.TempFile(directoryPath, re.TestPlan)
	defer file.Close()
	if err != nil {
		return "", err
	}

	buf := bufio.NewWriter(file)
	var written int64
	for written < size {
		w, err := io.CopyN(buf, rand.Reader, size-written)
		if err != nil {
			return "", err
		}
		written += w
	}

	err = buf.Flush()
	if err != nil {
		return "", err
	}

	return file.Name(), file.Sync()
}

// CreateRandomDirectory creates a nested directory with the specified depth within the specified
// directory path. If depth is zero, the directory path is returned.
func (re *RunEnv) CreateRandomDirectory(directoryPath string, depth uint) (string, error) {
	if depth == 0 {
		return directoryPath, nil
	}

	base, err := ioutil.TempDir(directoryPath, re.TestPlan)
	if err != nil {
		return "", err
	}

	name := base
	var i uint
	for i = 1; i < depth; i++ {
		name, err = ioutil.TempDir(name, "tg")
		if err != nil {
			return "", err
		}
	}

	return base, nil
}

// CreateAsset creates an output asset.
//
// Output assets will be saved when the test terminates and available for
// further investigation. You can also manually create output assets/directories
// under re.TestOutputsPath.
func (re *RunEnv) CreateAsset(name string) (*os.File, error) {
	return os.Create(filepath.Join(re.TestOutputsPath, name))
}
