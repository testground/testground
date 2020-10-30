package metrics

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	client "github.com/influxdata/influxdb1-client/v2"
	"github.com/testground/testground/pkg/config"
)

var tagsIgnoreList = map[string]struct{}{}

var previousDays = "7d"

func init() {
	tagsIgnoreList["plan"] = struct{}{}
	tagsIgnoreList["case"] = struct{}{}
	tagsIgnoreList["group_id"] = struct{}{}
	tagsIgnoreList["run"] = struct{}{}
}

type Viewer struct {
	db string
	cl client.Client
}

type Row struct {
	Run       string
	Timestamp string
	Fields    map[string]json.Number // tag variation -> value
}

func NewViewer(cfg *config.EnvConfig) (*Viewer, error) {
	cl, err := client.NewHTTPClient(client.HTTPConfig{
		Addr: cfg.Daemon.InfluxDBEndpoint,
	})
	if err != nil {
		return nil, err
	}
	return &Viewer{db: "testground", cl: cl}, nil
}

func (v *Viewer) GetMeasurements(name string) ([]string, error) {
	cmd := fmt.Sprintf("SHOW MEASUREMENTS ON testground WITH MEASUREMENT =~ /results.%s.*/ LIMIT 20", name)

	q := client.Query{
		Command:  cmd,
		Database: v.db,
	}

	response, err := v.cl.Query(q)
	if err != nil {
		return nil, err
	}

	if response.Error() != nil {
		return nil, response.Error()
	}

	var measurements []string

	if response.Results == nil || response.Results[0].Series == nil {
		return nil, nil
	}

	series := response.Results[0].Series[0].Values
	for _, s := range series {
		m := s[0].(string)

		measurements = append(measurements, m)
	}

	return measurements, nil
}

func (v *Viewer) GetTags(series string) ([]string, error) {
	cmd := fmt.Sprintf("SHOW TAG KEYS ON %s FROM \"%s\"", v.db, series)
	q := client.Query{
		Command:  cmd,
		Database: v.db,
	}

	response, err := v.cl.Query(q)
	if err != nil {
		return nil, err
	}

	if response.Error() != nil {
		return nil, response.Error()
	}

	var tags []string

	values := response.Results[0].Series[0].Values
	for _, v := range values {
		vs := v[0].(string)

		if _, ok := tagsIgnoreList[vs]; ok {
			continue
		}

		tags = append(tags, vs)
	}

	return tags, nil
}

func (v *Viewer) GetTagsValues(tags []string) (map[string][]string, error) {
	tagsValues := map[string][]string{}

	for _, t := range tags {
		cmd := fmt.Sprintf("SHOW TAG VALUES ON %s WITH KEY = \"%s\" WHERE time > now()-%s", v.db, t, previousDays)

		q := client.Query{
			Command:  cmd,
			Database: v.db,
		}

		response, err := v.cl.Query(q)
		if err != nil {
			return nil, err
		}

		if response.Error() != nil {
			return nil, response.Error()
		}

		values := response.Results[0].Series[0].Values

		for _, v := range values {
			key := v[0].(string)
			value := v[1].(string)

			tagsValues[key] = append(tagsValues[key], value)
		}
	}

	return tagsValues, nil
}

func (v *Viewer) GetData(series string, tags []string, tagsWithValues map[string][]string) (map[string]Row, []string, []string, error) {
	// get timestamps for runs
	rows := map[string]Row{}
	var marshaledTags []string
	var orderedRuns []string
	{
		cmd := fmt.Sprintf("SELECT last(\"value\"), \"run\" FROM \"%s\" WHERE time > now()-%s GROUP BY \"run\"", series, previousDays)

		q := client.Query{
			Command:  cmd,
			Database: v.db,
		}

		response, err := v.cl.Query(q)
		if err != nil {
			return nil, nil, nil, err
		}

		if response.Error() != nil {
			return nil, nil, nil, response.Error()
		}

		data := response.Results[0].Series

		for _, row := range data {
			var r Row

			r.Run = row.Tags["run"]
			r.Timestamp = row.Values[0][0].(string)
			r.Fields = make(map[string]json.Number)

			rows[r.Run] = r
			orderedRuns = append(orderedRuns, r.Run)
		}
	}

	var lastRun string
	// get all fields for each row (which is a run)
	{
		for i := range tags {
			tags[i] = "\"" + tags[i] + "\""
		}

		tags = append(tags, "\"run\"")

		t := strings.Join(tags, ",")

		cmd := fmt.Sprintf("SELECT mean(\"value\") FROM \"%s\" WHERE time > now()-%s GROUP BY %s", series, previousDays, t)

		q := client.Query{
			Command:  cmd,
			Database: v.db,
		}

		response, err := v.cl.Query(q)
		if err != nil {
			return nil, nil, nil, err
		}

		data := response.Results[0].Series

		for _, row := range data {
			run := row.Tags["run"]

			if _, ok := rows[run]; !ok {
				panic("cant find run, something is wrong")
			}

			val := row.Values[0][1].(json.Number)
			marshaledTag := marshalTags(row.Tags)

			if marshaledTag == "" {
				marshaledTag = "value"
			}

			rows[run].Fields[marshaledTag] = val

			lastRun = run
		}
	}

	// fetch marshaledTags based on last run
	{
		fields := rows[lastRun].Fields

		keys := make([]string, 0, len(fields))
		for k := range fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		marshaledTags = keys
	}

	return rows, marshaledTags, orderedRuns, nil
}

func marshalTags(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var result []string
	for _, k := range keys {
		if k == "run" {
			continue
		}
		result = append(result, fmt.Sprintf("%s=%s", k, m[k]))
	}

	return strings.Join(result, ",")
}
