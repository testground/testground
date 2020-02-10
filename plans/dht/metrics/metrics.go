package metrics

import (
	"os"
	"time"

	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/metrics/influxdb"
)

var (
	endpoint  = "http://influxdb:8086"
	database  = "metrics"
	username  = ""
	password  = ""
	namespace = ""
	tags      = map[string]string{}
)

func Setup() {
	metrics.Enabled = true
	hostname, _ := os.Hostname()
	tags["host"] = hostname
	go influxdb.InfluxDBWithTags(metrics.DefaultRegistry, 10*time.Second, endpoint, database, username, password, namespace, tags)
}

func EmitMetrics() error {
	return influxdb.InfluxDBWithTagsOnce(metrics.DefaultRegistry, endpoint, database, username, password, namespace, tags)
}
