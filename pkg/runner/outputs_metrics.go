package runner

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ipfs/testground/pkg/rpc"
	"github.com/ipfs/testground/sdk/runtime"

	"github.com/influxdata/influxdb-client-go"
)

// This is used to unmarshal the zap log
// TODO Unmarshal using the actual zap logger
type logRow struct {
	Ts      int64          `json:"ts,omitempty"`
	Msg     string         `json:"msg,omitempty"`
	GroupId string         `json:"group_id,omitempty"`
	RunId   string         `json:"run_id,omitempty"`
	Event   *runtime.Event `json:"event,omitempty"`
}

// filterMetrics is processes a single run.out file. When it finds a Metric type,
// it writes the the unmarshaled log row to the channel.
func filterMetrics(buf *bytes.Buffer, wg *sync.WaitGroup, rowCh chan logRow) {
	var row logRow
	scnr := bufio.NewScanner(buf)
	for scnr.Scan() {
		err := json.Unmarshal(scnr.Bytes(), &row)
		if err != nil {
			continue
		}
		event := *(row.Event)
		if event.Type != runtime.EventTypeMetric {
			continue
		}
		rowCh <- row
	}
	wg.Done()
}

// eventRecorder creates an influx client. This method creates points from log rows it receives from
// the channel.
func eventRecorder(rowCh chan logRow, doneCh chan int, ow *rpc.OutputWriter, url string, token string, org string, bucket string) {
	client := influxdb2.NewClient(url, token)
	defer client.Close()
	writeApi := client.WriteApi(org, bucket)
	go func() {
		for err := range writeApi.Errors() {
			ow.Warnw("Error writing message to influx", "err", err)
		}
	}()
	for row := range rowCh {
		// Pull out the important bits of the log message and create an influxdb event
		event := *(row.Event)
		timestamp := row.Ts
		measurement := fmt.Sprintf("%s (%s)", event.Metric.Name, event.Metric.Unit)

		// Unfortunately? the gitsha, etc is not included in the logs.
		// Hopefully the GroupID is sufficient to distinguish between code versions
		tags := map[string]string{
			"GroupID": row.GroupId,
			"RunID":   row.RunId,
		}

		fields := map[string]interface{}{
			event.Metric.Unit: event.Metric.Value,
		}
		pt := influxdb2.NewPoint(measurement, tags, fields, time.Unix(timestamp, 0))
		writeApi.WritePoint(pt)
	}
	writeApi.Flush()
	close(doneCh)

}

// MetricsWalkTarfilea takes a Reader which should be a gzipped tarball. This is the file format
// used for outputs. This function creates buffers for each file in the tar file and uses it
// to generate metrics. The collection tarfile consists of files named "run.out" and "run.err".
// This method filters out error files.
func MetricsWalkTarfile(src io.Reader, ow *rpc.OutputWriter, url string, token string, org string, bucket string) {
	rowCh := make(chan logRow)
	doneCh := make(chan int)
	ow.Info("Uploading events to %s", url)
	go eventRecorder(rowCh, doneCh, ow, url, token, org, bucket)

	dec, err := gzip.NewReader(src)
	if err != nil {
		panic(err)
	}
	defer dec.Close()

	tf := tar.NewReader(dec)

	var wg sync.WaitGroup
	for hdr, err := tf.Next(); err != io.EOF; hdr, err = tf.Next() {
		if err != nil {
			panic(err)
		}

		fi := hdr.FileInfo()
		if fi.IsDir() || fi.Name() == "run.err" {
			continue
		}

		s := make([]byte, hdr.Size)
		buf := bytes.NewBuffer(s)
		_, err := io.Copy(buf, tf)
		if err != nil {
			panic(err)
		}
		wg.Add(1)
		go filterMetrics(buf, &wg, rowCh)
	}
	ow.Info("waiting for filterMetrics runners")
	wg.Wait()
	close(rowCh)
	ow.Info("waiting for eventRecorder")
	<-doneCh
	ow.Info("metrics upload complete")
}
