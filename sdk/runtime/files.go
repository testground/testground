package runtime

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// CreateRandomFile creates a file of the specified size (in bytes) within the
// specified directory path and returns its path.
func (re *RunEnv) CreateRandomFile(directoryPath string, size int64) (string, error) {
	file, err := ioutil.TempFile(directoryPath, re.TestPlan)
	if err != nil {
		return "", err
	}
	defer file.Close()

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

// CreateRawAsset creates an output asset.
//
// Output assets will be saved when the test terminates and available for
// further investigation. You can also manually create output assets/directories
// under re.TestOutputsPath.
func (re *RunEnv) CreateRawAsset(name string) (*os.File, error) {
	var file *os.File
	err := retry(5, re.retryDumpDirectoryStat(re.TestOutputsPath, func() error {
		var err error
		file, err = os.Create(filepath.Join(re.TestOutputsPath, name))
		return err
	}))

	if err != nil {
		return nil, err
	}

	select {
	case re.unstructured <- file:
	default:
		return nil, fmt.Errorf("too many unstructured assets; current: %d", len(re.unstructured))
	}

	return file, nil
}

func (re *RunEnv) retryDumpDirectoryStat(path string, fn func() error) func() error {
	return func() error {
		err := fn()
		if err == nil {
			return nil
		}

		f, staterr := os.Stat(path)
		spew.Dump(err, f, staterr)

		re.RecordMessage("failed finding dir: err %v, stat %v with staterr %v", err, f, staterr)
		return err
	}
}

func retry(tries int, fn func() error) error {
	var err error
	for i := 0; i < tries; i++ {
		err = fn()
		if err == nil {
			break
		}
	}
	return err
}

// CreateStructuredAsset creates an output asset and wraps it in zap loggers.
func (re *RunEnv) CreateStructuredAsset(name string, config zap.Config) (*zap.Logger, *zap.SugaredLogger, error) {
	path := filepath.Join(re.TestOutputsPath, name)
	config.OutputPaths = []string{path}

	var logger *zap.Logger
	err := retry(5, re.retryDumpDirectoryStat(re.TestOutputsPath, func() error {
		var err error
		logger, err = config.Build()
		return err
	}))

	if err != nil {
		return nil, nil, err
	}

	select {
	case re.structured <- logger:
	default:
		return nil, nil, fmt.Errorf("too many structured assets; current: %d", len(re.structured))
	}

	return logger, logger.Sugar(), nil
}

// StandardJSONConfig returns a zap.Config with JSON encoding, debug verbosity,
// caller and stacktraces disabled, and with timestamps encoded as nanos after
// epoch.
func StandardJSONConfig() zap.Config {
	enc := zap.NewProductionEncoderConfig()
	enc.EncodeTime = zapcore.EpochNanosTimeEncoder

	return zap.Config{
		Level:             zap.NewAtomicLevelAt(zap.DebugLevel),
		Encoding:          "json",
		EncoderConfig:     enc,
		DisableCaller:     true,
		DisableStacktrace: true,
	}
}
