package utils

import (
	"bufio"
	"crypto/rand"
	"io"
	"io/ioutil"
	"os"

	"github.com/ipfs/testground/sdk/runtime"
)

// CreateRandomFile creates a file of the specified size (in bytes) within the
// specified directory path
func CreateRandomFile(runenv *runtime.RunEnv, directoryPath string, size int64) (*os.File, error) {
	file, err := ioutil.TempFile(directoryPath, runenv.TestPlan)
	if err != nil {
		return nil, err
	}

	buf := bufio.NewWriter(file)
	var written int64
	for written < size {
		w, err := io.CopyN(buf, rand.Reader, size-written)
		if err != nil {
			return nil, err
		}
		written += w
	}

	err = buf.Flush()
	if err != nil {
		return nil, err
	}

	err = file.Sync()
	if err != nil {
		return nil, err
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	return file, nil
}
