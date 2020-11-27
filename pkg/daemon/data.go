package daemon

import (
	"encoding/csv"
	"fmt"
	"net/http"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/metrics"
)

func (d *Daemon) dataHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logging.S().With("req_id", r.Header.Get("X-Request-ID"))

		log.Debugw("handle request", "command", "dashboard task")
		defer log.Debugw("request handled", "command", "dashboard task")

		w.Header().Set("Content-Type", "text/plain")
		//enableCors(&w)

		series := r.URL.Query().Get("series")
		if series == "" {
			fmt.Fprintf(w, "query param `series` is missing")
			return
		}

		tags, err := d.mv.GetTags(series)
		if err != nil {
			fmt.Fprintf(w, "failed to get tags for series %s: %s", series, err)
			return
		}

		tagsWithValues, err := d.mv.GetTagsValues(tags)
		if err != nil {
			fmt.Fprintf(w, "failed to get tags values for series %s: %s", series, err)
			return
		}

		data, marshaledTags, orderedRuns, err := d.mv.GetData(series, tags, tagsWithValues)
		if err != nil {
			fmt.Fprintf(w, "failed to get data for series %s: %s", series, err)
			return
		}

		csvData := generateCsvData(data, orderedRuns, marshaledTags)

		cw := csv.NewWriter(w)
		for _, row := range csvData {
			_ = cw.Write(row)
		}
		cw.Flush()

		log.Debugw("done processing tasks", "series", series)
	}
}

func generateCsvData(data map[string]metrics.Row, runs []string, tags []string) [][]string {
	result := [][]string{}
	firstLine := append([]string{"Time"}, tags...)

	result = append(result, firstLine)

	for _, r := range runs {
		v := data[r]

		line := []string{v.Timestamp}

		allEmpty := true
		for _, t := range tags {
			entry, ok := v.Fields[t]
			if !ok {
				line = append(line, "")
			} else {
				line = append(line, entry.String())
				allEmpty = false
			}
		}

		if !allEmpty {
			result = append(result, line)
		}
	}

	return result
}
