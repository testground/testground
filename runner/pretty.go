package runner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/testground/testground/logging"
	"github.com/testground/testground/rpc"
	"github.com/testground/sdk-go/runtime"

	"github.com/logrusorgru/aurora"
)

type eventType int

const (
	Error eventType = iota
	Start
	Ok
	Fail
	Crash
	Incomplete
	Message
	Metric
	Other
	InternalErr
)

func (et eventType) String() string {
	return [...]string{"Error", "Start", "Ok", "Fail", "Crash", "Incomplete", "Message", "Metric", "Other", "InternalErr"}[et]
}

// PrettyPrinter is a logger that sends output to the console.
type PrettyPrinter struct {
	aurora  aurora.Aurora
	classes [10]aurora.Value
	ow      *rpc.OutputWriter

	// guarded by atomic.
	failed uint32
	count  uint32

	start time.Time
	wg    sync.WaitGroup
}

// NewPrettyPrinter constructs a new console logger.
func NewPrettyPrinter(ow *rpc.OutputWriter) *PrettyPrinter {
	au := aurora.NewAurora(logging.IsTerminal())
	return &PrettyPrinter{
		aurora: au,
		classes: [...]aurora.Value{
			aurora.BgRed("ERROR").White(),
			aurora.BgBrightCyan("START").Black(),
			aurora.BgGreen("OK").White(),
			aurora.BgRed("FAIL").White(),
			aurora.BgBrightRed("CRASH").White(),
			aurora.BgBrightRed("INCOMPLETE").White(),
			aurora.BgWhite("MESSAGE").Black(),
			aurora.BgBlue("METRIC").White(),
			aurora.BgMagenta("OTHER").White(),
			aurora.BgBrightRed("INTERNAL_ERR").White(),
		},
		start: time.Now(),
		ow:    ow,
	}
}

// Wait waits for all running tests to finish and returns an error if any of
// them failed.
func (c *PrettyPrinter) Wait() <-chan error {
	ch := make(chan error)
	go func() {
		c.wg.Wait()
		if f := atomic.LoadUint32(&c.failed); f > 0 {
			ch <- fmt.Errorf("%d nodes failed", f)
		}
		ch <- nil
	}()
	return ch
}

// FailStart should be used to report that an instance failed to start.
func (c *PrettyPrinter) FailStart(id string, message interface{}) {
	cnt := atomic.AddUint32(&c.count, 1)
	atomic.AddUint32(&c.failed, 1)
	c.print(cnt-1, id, time.Now(), Incomplete, "failed to start:", message)
}

// processStderr processes unstructured log output that's not managed by zap, in
// a line-by-line fashion.
func (c *PrettyPrinter) processStderr(idx uint32, id string, stderr io.ReadCloser) {
	defer stderr.Close()

	scanner := bufio.NewScanner(stderr)

	for scanner.Scan() {
		c.print(idx, id, time.Now(), Error, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		c.print(idx, id, time.Now(), Error, err)
	}
}

// processStdout processes structured log output managed by zap.
func (c *PrettyPrinter) processStdout(idx uint32, id string, stdout io.ReadCloser) {
	defer stdout.Close()

	var (
		failed, ok bool
		all        = make(map[string]json.RawMessage, 16)
	)

	defer func() {
		if !ok && !failed {
			// incomplete.
			c.print(idx, id, time.Now(), Incomplete)
		}
		if !ok || failed {
			atomic.AddUint32(&c.failed, 1)
		}
	}()

	for scanner := bufio.NewScanner(stdout); scanner.Scan(); {
		// clear the map (optimized by the compiler).
		for k := range all {
			delete(all, k)
		}

		line := scanner.Bytes()

		// decode the incoming log line.
		switch err := json.Unmarshal(line, &all); err {
		case nil:
		case io.EOF, context.Canceled:
			return
		default:
			c.print(idx, id, time.Now(), Other, string(line))
			continue
		}

		var (
			evt runtime.Event
			ts  time.Time
		)

		var nanos int64
		_ = json.Unmarshal(all["ts"], &nanos)
		ts = time.Unix(0, nanos)

		if err := json.Unmarshal(all["event"], &evt); err != nil {
			c.print(idx, id, time.Now(), Other, string(line))
			continue
		}

		switch evt.Type {
		case runtime.EventTypeFinish:
			switch evt.Outcome {
			case runtime.EventOutcomeOK:
				ok = true
				c.print(idx, id, ts, Ok, "")
			case runtime.EventOutcomeFailed:
				failed = true
				c.print(idx, id, ts, Fail, evt.Error)
			case runtime.EventOutcomeCrashed:
				failed = true
				c.print(idx, id, ts, Crash, evt.Error, evt.Stacktrace)
			default:
				c.print(idx, id, ts, InternalErr, fmt.Sprintf("unknown outcome: %s", evt.Outcome))
				return
			}

		case runtime.EventTypeMetric:
			m, _ := json.Marshal(evt.Metric)
			c.print(idx, id, ts, Metric, string(m))

		case runtime.EventTypeMessage:
			c.print(idx, id, ts, Message, evt.Message)

		case runtime.EventTypeStart:
			m, _ := json.Marshal(evt.Runenv)
			c.print(idx, id, ts, Start, string(m))
		}
	}
}

// Manage should be called on the standard output of all instances. It will
// send the events to a logger and record whether or not the test passed.
func (c *PrettyPrinter) Manage(id string, stdout, stderr io.ReadCloser) {
	idx := atomic.AddUint32(&c.count, 1) - 1

	c.wg.Add(2)
	go func() {
		defer c.wg.Done()
		c.processStderr(idx, id, stderr)
	}()

	go func() {
		defer c.wg.Done()
		c.processStdout(idx, id, stdout)
	}()
}

func (c *PrettyPrinter) print(idx uint32, id string, now time.Time, evtType eventType, message ...interface{}) {
	var (
		elapsed = now.Sub(c.start)
		class   = c.classes[evtType]
		msg     = fmt.Sprint(message...)
	)

	if elapsed < 0 {
		elapsed = 0
	}

	c.ow.Infof("%5.4fs %10s %s %s",
		float64(elapsed)/float64(time.Second),
		class,
		c.aurora.Index(uint8(idx%15)+1, "<< "+id+" >>"),
		msg,
	)
}
