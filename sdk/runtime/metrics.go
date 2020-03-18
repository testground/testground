package runtime

import (
	"context"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func NewCounter(runenv *RunEnv, name string, help string) prometheus.Counter {
	c := prometheus.NewCounter(prometheus.CounterOpts{
		Name: name,
		Help: help,
	})
	runenv.MetricsPusher.Collector(c)
	return c
}

func NewGauge(runenv *RunEnv, name string, help string) prometheus.Gauge {
	g := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: name,
		Help: help,
	})
	runenv.MetricsPusher.Collector(g)
	return g
}

func NewHistogram(runenv *RunEnv, name string, help string, buckets ...float64) prometheus.Histogram {
	h := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    name,
		Help:    help,
		Buckets: buckets,
	})
	runenv.MetricsPusher.Collector(h)
	return h
}

func NewSummary(runenv *RunEnv, name string, help string, opts prometheus.SummaryOpts) prometheus.Summary {
	opts.Name = name
	opts.Help = help
	s := prometheus.NewSummary(opts)
	runenv.MetricsPusher.Collector(s)
	return s
}

// HTTPPeriodicSnapshots periodically fetches the snapshots from the given address
// and outputs them to the out directory. Every file will be in the format timestamp.out.
func (re *RunEnv) HTTPPeriodicSnapshots(ctx context.Context, addr string, dur time.Duration, outDir string) error {
	err := os.MkdirAll(path.Join(re.TestOutputsPath, outDir), 0777)
	if err != nil {
		return err
	}

	nextFile := func() (*os.File, error) {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		return os.Create(path.Join(re.TestOutputsPath, outDir, timestamp+".out"))
	}

	go func() {
		ticker := time.NewTicker(dur)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				func() {
					req, err := http.NewRequestWithContext(ctx, "GET", addr, nil)
					if err != nil {
						re.RecordMessage("error while creating http request: %v", err)
						return
					}

					resp, err := http.DefaultClient.Do(req)
					if err != nil {
						re.RecordMessage("error while scraping http endpoint: %v", err)
						return
					}
					defer resp.Body.Close()

					file, err := nextFile()
					if err != nil {
						re.RecordMessage("error while getting metrics output file: %v", err)
						return
					}
					defer file.Close()

					_, err = io.Copy(file, resp.Body)
					if err != nil {
						re.RecordMessage("error while copying data to file: %v", err)
						return
					}
				}()
			}
		}
	}()

	return nil
}
