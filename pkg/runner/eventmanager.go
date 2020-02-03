package runner

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ipfs/testground/sdk/runtime"

	"go.uber.org/zap/zapcore"
)

type eventType int

const (
	Error eventType = iota
	Ok
	Fail
	Crash
	Incomplete
	Message
	Metric
)

func (et eventType) String() string {
	return [...]string{"Error", "Ok", "Fail", "Crash", "Incomplete", "Message", "Metric"}[et]
}

// eventLogger logs events to the console / a file / etc
type eventLogger interface {
	// msg logs a message
	msg(idx int, id string, elapsed time.Duration, evtType eventType, message ...interface{})
	// metric logs a metric
	metric(idx int, id string, elapsed time.Duration, metric *runtime.Metric)
	// sync is called just before logging completes
	sync()
}

// EventManager is a helper type for collecting test output and sending it to a logger.
type EventManager struct {
	failed uint32

	count  int
	start  time.Time
	wg     sync.WaitGroup
	logger eventLogger
}

// NewEventManager constructs a new event manager.
func NewEventManager(logger eventLogger) *EventManager {
	return &EventManager{
		start:  time.Now(),
		logger: logger,
	}
}

// Wait waits for all running tests to finish and returns an error if any of
// them failed.
func (c *EventManager) Wait() error {
	c.wg.Wait()
	if c.failed > 0 {
		return fmt.Errorf("%d nodes failed", c.failed)
	}
	return nil
}

// FailStart should be used to report that a test failed to start.
func (c *EventManager) FailStart(id string, message interface{}) {
	idx := c.count
	c.count++
	atomic.AddUint32(&c.failed, 1)
	c.msg(idx, id, time.Now(), Incomplete, "failed to start:", message)
}

// Manage should be called on the standard output of all test plans. It will
// send the events to a logger and record whether or not the test passed.
func (c *EventManager) Manage(id string, stdout, stderr io.ReadCloser) {
	idx := c.count
	c.count++

	printMsg := func(timestamp int64, evtType eventType, message ...interface{}) {
		now := time.Unix(0, timestamp)
		c.msg(idx, id, now, evtType, message...)
	}

	c.wg.Add(2)
	go func() {
		defer stderr.Close()
		defer c.wg.Done()

		decoder := json.NewDecoder(stderr)
		for {
			event := make(map[string]json.RawMessage, 16)
			if err := decoder.Decode(&event); err != nil {
				if err != io.EOF {
					now := time.Now().UnixNano()
					printMsg(now, Error, "stderr error: "+err.Error())
				}
				return
			}

			var (
				level               zapcore.Level
				levelS              string
				ts                  time.Time
				name, code, message string
			)

			// Ignore everything less than a warning.
			// Unless we can't parse the level.
			_ = json.Unmarshal(event["L"], &levelS)
			if err := level.Set(levelS); err == nil && level < zapcore.WarnLevel {
				continue
			}

			// Dealing with errors just isn't worth it.
			_ = json.Unmarshal(event["T"], &ts)
			_ = json.Unmarshal(event["N"], &name)
			_ = json.Unmarshal(event["C"], &code)
			_ = json.Unmarshal(event["M"], &message)

			// Filter down to the custom fields.
			for _, k := range [...]string{
				"L", "T", "N", "C", "M", "S",
				"plan", "case", "run", "seq",
				"repo", "commit", "branch",
				"tag", "instances",
			} {
				delete(event, k)
			}

			fields, _ := json.Marshal(event)
			c.msg(idx, id, ts, Error, fmt.Sprintf("%s %s %s\t%s", code, name, message, string(fields)))
		}
	}()

	go func() {
		defer stdout.Close()
		defer c.wg.Done()
		defer c.logger.sync()

		decoder := json.NewDecoder(stdout)
		var (
			// track both in case a test-case is so broken that it
			// reports both a success and a failure.
			failed = false
			ok     = false
		)

		for {
			var event runtime.Event
			if err := decoder.Decode(&event); err != nil {
				now := time.Now().UnixNano()
				if err != io.EOF {
					printMsg(now, Error, "stdout error: "+err.Error())
					failed = true
				}
				if !ok && !failed {
					// incomplete.
					printMsg(event.Timestamp, Incomplete)
				}

				if !ok || failed {
					atomic.AddUint32(&c.failed, 1)
				}

				return
			}

			if event.Result != nil {
				switch event.Result.Outcome {
				case runtime.OutcomeOK:
					ok = true
					printMsg(event.Timestamp, Ok, event.Result.Reason)
				case runtime.OutcomeCrashed:
					failed = true
					printMsg(event.Timestamp, Crash, event.Result.Outcome, " ", event.Result.Reason, event.Result.Stack)
				case runtime.OutcomeAborted:
					failed = true
					printMsg(event.Timestamp, Fail, event.Result.Outcome, " ", event.Result.Reason)
				default:
					panic(fmt.Sprintf("unknown outcome: %s", event.Result.Outcome))
				}
			} else if event.Metric != nil {
				now := time.Unix(0, event.Timestamp)
				c.logger.metric(idx, id, c.getElapsed(now), event.Metric)
			} else {
				printMsg(event.Timestamp, Message, event.Message)
			}
		}
	}()
}

func (c *EventManager) msg(idx int, id string, now time.Time, evtType eventType, message ...interface{}) {
	c.logger.msg(idx, id, c.getElapsed(now), evtType, message...)
}

func (c *EventManager) getElapsed(now time.Time) time.Duration {
	elapsed := now.Sub(c.start)
	if elapsed < 0 {
		elapsed = 0
	}
	return elapsed
}
