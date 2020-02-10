package metrics

import (
	"fmt"
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
	namespace = "testplan"
	tags      = map[string]string{
		"testplan": "dht",
	}
)

func Setup() {
	fmt.Println("setting up metrics")
	hostname, _ := os.Hostname()
	tags["host"] = hostname
	go influxdb.InfluxDBWithTags(metrics.DefaultRegistry, 5*time.Second, endpoint, database, username, password, namespace, tags)
}

func EmitMetrics() error {
	return influxdb.InfluxDBWithTagsOnce(metrics.DefaultRegistry, endpoint, database, username, password, namespace, tags)
}
