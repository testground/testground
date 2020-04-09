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
func filterMetrics(buf *bytes.Buffer, wg *sync.WaitGroup, ow *rpc.OutputWriter, rowCh chan logRow) {
	var row logRow
	scnr := bufio.NewScanner(buf)
	for scnr.Scan() {
		err := json.Unmarshal(scnr.Bytes(), &row)
		if err != nil {
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
	// Hitting the write limit is very easy.
	// Free influxdb cloud account is 5.1 MB/ 5 minutes.
	// each point is about 250 bytes. We can send about 4080 points per minute
	var counter int
	var size int
	ticker := time.Tick(time.Second / 68)
	for row := range rowCh {
		<-ticker
		counter++
		// approximate size
		// Pull out the important bits of the log message and create an influxdb event
		event := *(row.Event)
		timestamp := row.Ts
		// This causes the measurements to look something like this:
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
		// Keep track of the size, approximately.
		size += len(measurement + row.GroupId + row.RunId)
		// timestamp is formatted as ssssssssssnnnnnnnnn
		// For example 1586397665879924824
		// Of course, just dividing this will not work forever.
		sec := timestamp / 1000000000
		nsec := timestamp % 1000000000
		pt := influxdb2.NewPoint(measurement, tags, fields, time.Unix(sec, nsec))
		writeApi.WritePoint(pt)
		// Make sure we flush sometimes.
		if counter%10000 == 0 {
			ow.Info(fmt.Sprintf("flushing about %d bytes to influxdb", size))
			writeApi.Flush()
			size = 0
		}
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
	ow.Info(fmt.Sprintf("Uploading events to %s", url))
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

		var buf bytes.Buffer
		_, err := io.Copy(&buf, tf)
		if err != nil {
			panic(err)
		}
		wg.Add(1)
		go filterMetrics(&buf, &wg, ow.With(hdr.Name), rowCh)
	}
	ow.Info("waiting for filterMetrics runners")
	wg.Wait()
	close(rowCh)
	ow.Info("waiting for eventRecorder")
	<-doneCh
	ow.Info("metrics upload complete")
}
