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

// Type aliases to hide implementation details in the APIs.
type (
	Counter   = prometheus.Counter
	Gauge     = prometheus.Gauge
	Histogram = prometheus.Histogram
	Summary   = prometheus.Summary

	CounterOpts   = prometheus.CounterOpts
	GaugeOpts     = prometheus.GaugeOpts
	HistogramOpts = prometheus.HistogramOpts
	SummaryOpts   = prometheus.SummaryOpts

	CounterVec   = prometheus.CounterVec
	GaugeVec     = prometheus.GaugeVec
	HistogramVec = prometheus.HistogramVec
	SummaryVec   = prometheus.SummaryVec
)

type Metrics struct {
	runenv *RunEnv
}

func (*Metrics) NewCounter(o CounterOpts) Counter {
	m := prometheus.NewCounter(o)
	switch err := prometheus.Register(m); err.(type) {
	case nil, prometheus.AlreadyRegisteredError:
	default:
		panic(err)
	}
	return m
}

func (*Metrics) NewGauge(o GaugeOpts) Gauge {
	m := prometheus.NewGauge(o)
	switch err := prometheus.Register(m); err.(type) {
	case nil, prometheus.AlreadyRegisteredError:
	default:
		panic(err)
	}
	return m
}

func (*Metrics) NewHistogram(o HistogramOpts) Histogram {
	m := prometheus.NewHistogram(o)
	switch err := prometheus.Register(m); err.(type) {
	case nil, prometheus.AlreadyRegisteredError:
	default:
		panic(err)
	}
	return m
}

func (*Metrics) NewSummary(o SummaryOpts) Summary {
	m := prometheus.NewSummary(o)
	switch err := prometheus.Register(m); err.(type) {
	case nil, prometheus.AlreadyRegisteredError:
	default:
		panic(err)
	}
	return m
}

func (*Metrics) NewCounterVec(o CounterOpts, labels ...string) *CounterVec {
	m := prometheus.NewCounterVec(o, labels)
	switch err := prometheus.Register(m); err.(type) {
	case nil, prometheus.AlreadyRegisteredError:
	default:
		panic(err)
	}
	return m
}

func (*Metrics) NewGaugeVec(o GaugeOpts, labels ...string) *GaugeVec {
	m := prometheus.NewGaugeVec(o, labels)
	switch err := prometheus.Register(m); err.(type) {
	case nil, prometheus.AlreadyRegisteredError:
	default:
		panic(err)
	}
	return m
}

func (*Metrics) NewHistogramVec(o HistogramOpts, labels ...string) *HistogramVec {
	m := prometheus.NewHistogramVec(o, labels)
	switch err := prometheus.Register(m); err.(type) {
	case nil, prometheus.AlreadyRegisteredError:
	default:
		panic(err)
	}
	return m
}

func (*Metrics) NewSummaryVec(o SummaryOpts, labels ...string) *SummaryVec {
	m := prometheus.NewSummaryVec(o, labels)
	switch err := prometheus.Register(m); err.(type) {
	case nil, prometheus.AlreadyRegisteredError:
	default:
		panic(err)
	}
	return m
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
