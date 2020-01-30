package runner

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/sdk/runtime"

	"github.com/logrusorgru/aurora"
)

// ConsoleLogger is a logger that sends output to the console.
type ConsoleLogger struct {
	aurora aurora.Aurora
}

// NewConsoleLogger constructs a new console logger.
func NewConsoleLogger() *ConsoleLogger {
	return &ConsoleLogger{
		aurora: aurora.NewAurora(logging.IsTerminal()),
	}
}

// log an event to the console
func (c *ConsoleLogger) msg(idx int, id string, elapsed time.Duration, evtType eventType, message ...interface{}) {
	kind := [...]aurora.Value{
		c.aurora.BgRed("ERROR").White(),
		c.aurora.BgGreen("OK").White(),
		c.aurora.BgRed("FAIL").White(),
		c.aurora.BgBrightRed("CRASH").White(),
		c.aurora.BgBrightRed("INCOMPLETE").White(),
		c.aurora.BgWhite("MESSAGE").Black(),
		c.aurora.BgBlue("METRIC").White(),
	}[evtType]

	msg := fmt.Sprint(message...)
	fmt.Printf("%9.4fs %10s %s %s\n",
		float64(elapsed)/float64(time.Second),
		kind,
		c.aurora.Index(uint8(idx%15)+1, "<< "+id+" >>"),
		msg,
	)
}

// log a metric to the console
func (c *ConsoleLogger) metric(idx int, id string, elapsed time.Duration, metric *runtime.Metric) {
	evtType := Metric
	var message string
	marshaled, err := json.Marshal(metric)
	if err != nil {
		evtType = Error
		message = fmt.Sprintf("malformed metric: %s", err)
	} else {
		message = string(marshaled)
	}
	c.msg(idx, id, elapsed, evtType, message)
}

func (c *ConsoleLogger) sync() {
}
