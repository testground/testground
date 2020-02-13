package runtime

import (
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
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
func (re *RunEnv) HTTPPeriodicSnapshots(addr string, dur time.Duration, out string) {
	includesPlaceholder := strings.Index(out, "$TIME") != -1
	nextFile := func() (*os.File, error) {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		filename := out + "-" + timestamp
		if includesPlaceholder {
			filename = strings.Replace(out, "$TIME", timestamp, 1)
		}

		return re.CreateRawAsset(filename)
	}

	for ; ; time.Sleep(dur) {
		resp, err := http.Get(addr)
		if err != nil {
			re.RecordMessage("error while getting prometheus stats: %v", err)
			continue
		}

		file, err := nextFile()
		if err != nil {
			re.RecordMessage("error while getting metrics output file: %v", err)
			continue
		}
		defer file.Close()

		io.Copy(file, resp.Body)
		time.Sleep(dur)
	}
}
