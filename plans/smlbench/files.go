package smlbench

import (
	"bufio"
	"context"
	"crypto/rand"
	"io"
	"io/ioutil"
	"os"

	"github.com/ipfs/testground/sdk/runtime"
)

// TempRandFile creates a file of the specified size (in bytes) within the
// specified directory.
//
// It is the callers responsibility to delete this file when done.
func TempRandFile(ctx context.Context, dir string, size int64) *os.File {
	re := runtime.CurrentRunEnv()
	file, err := ioutil.TempFile(dir, re.TestPlan)
	if err != nil {
		panic(err)
	}

	buf := bufio.NewWriter(file)
	var written int64
	for written < size {
		w, err := io.CopyN(buf, rand.Reader, size-written)
		if err != nil {
			panic(err)
		}
		written += w
	}

	err = buf.Flush()
	if err != nil {
		panic(err)
	}

	err = file.Sync()
	if err != nil {
		panic(err)
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		panic(err)
	}

	return file
}
