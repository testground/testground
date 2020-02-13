package runtime

import (
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MustExportPrometheus starts an HTTP server with the Prometheus handler.
// It starts on a random open port and returns the http address.
func (re *RunEnv) MustExportPrometheus() string {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}

	go http.Serve(listener, promhttp.Handler())

	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	return "http://127.0.0.1:" + port
}

// HTTPPeriodicSnapshots periodically fetches the snapshots from the given address
// and outputs them to the out file. The out filename may contain a $TIME placeholder.
// Otherwise, the time will just be appended to it.
func (re *RunEnv) HTTPPeriodicSnapshots(addr string, dur time.Duration, directory string) {
	err := os.MkdirAll(path.Join(re.TestOutputsPath, directory), 0777)
	if err != nil {
		re.RecordMessage("cannot create metrics directory: %v", err)
		return
	}

	nextFile := func() (*os.File, error) {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		return os.Create(path.Join(re.TestOutputsPath, directory, timestamp+".out"))
	}

	for ; ; time.Sleep(dur) {
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
			time.Sleep(dur)
		}()
	}
}
