package sidecar

import (
	"io"

	"github.com/docker/docker/pkg/stdcopy"
	"github.com/hashicorp/go-multierror"
)

type dockerLogs struct { //nolint
	logs           io.ReadCloser
	stdout, stderr io.Reader
	done           chan struct{}
	err            error
}

func newDockerLogs(logs io.ReadCloser) *dockerLogs { //nolint
	rstderr, wstderr := io.Pipe()
	rstdout, wstdout := io.Pipe()

	dl := &dockerLogs{
		logs:   logs,
		stdout: rstdout,
		stderr: rstderr,
		done:   make(chan struct{}),
	}
	go func() {
		_, dl.err = stdcopy.StdCopy(wstdout, wstderr, logs)
		close(dl.done)
	}()

	return dl
}

func (dl *dockerLogs) Stdout() io.Reader {
	return dl.stdout
}

func (dl *dockerLogs) Stderr() io.Reader {
	return dl.stderr
}

func (dl *dockerLogs) Close() error {
	var err *multierror.Error
	err = multierror.Append(err, dl.logs.Close())
	err = multierror.Append(err, dl.Wait())
	return err.ErrorOrNil()
}

func (dl *dockerLogs) Wait() error {
	<-dl.done
	return dl.err
}
