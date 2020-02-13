package runtime

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MustExportPrometheus starts an HTTP server with the Prometheus handler.
// It starts on a random open port and returns the listener. It is the caller
// responsability to close the listener.
func (re *RunEnv) MustExportPrometheus() net.Listener {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	go http.Serve(listener, promhttp.Handler())
	return listener
}

// HTTPPeriodicSnapshots periodically fetches the snapshots from the given address
// and outputs them to the out directory. Every file will be in the format timestamp.out.
func (re *RunEnv) HTTPPeriodicSnapshots(ctx context.Context, addr string, dur time.Duration, outDir string) error {
	if !strings.HasPrefix(addr, "http") {
		addr = "http://" + addr
	}

	err := os.MkdirAll(path.Join(re.TestOutputsPath, outDir), 0777)
	if err != nil {
		return err
	}

	nextFile := func() (*os.File, error) {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		return os.Create(path.Join(re.TestOutputsPath, outDir, timestamp+".out"))
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				func() {
					resp, err := http.Get(addr)
					if err != nil {
						re.RecordMessage("error while getting prometheus stats: %v", err)
						return
					}
					defer resp.Body.Close()

					file, err := nextFile()
					if err != nil {
						re.RecordMessage("error while getting metrics output file: %v", err)
						return
					}
					defer file.Close()

					io.Copy(file, resp.Body)
				}()
			}

			time.Sleep(dur)
		}
	}()

	return nil
}
