package runner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/sdk/runtime"

	"github.com/logrusorgru/aurora"
)

// ConsoleOutput is a helper type for collecting test output and sending it to the console.
type ConsoleOutput struct {
	failed uint32

	count  int
	start  time.Time
	aurora aurora.Aurora
	wg     sync.WaitGroup
}

// NewConsoleOutput constructs a new console output manager.
func NewConsoleOutput() *ConsoleOutput {
	return &ConsoleOutput{
		start:  time.Now(),
		aurora: aurora.NewAurora(logging.IsTerminal()),
	}
}

// Wait waits for all running tests to finish and returns an error if any of
// them failed.
func (c *ConsoleOutput) Wait() error {
	c.wg.Wait()
	if c.failed > 0 {
		return fmt.Errorf("%d nodes failed", c.failed)
	}
	return nil
}

func (c *ConsoleOutput) msg(idx int, id string, now time.Time, kind interface{}, message ...interface{}) {
	eventTime := now.Sub(c.start)
	if eventTime < 0 {
		eventTime = 0
	}
	fmt.Printf("%s\t%10s %s %s\n",
		eventTime,
		kind,
		c.aurora.Index(uint8(idx%15)+1, "<< "+id+" >>"),
		fmt.Sprint(message...),
	)
}

// FailStart should be used to report that a test failed to start.
func (c *ConsoleOutput) FailStart(id string, message interface{}) {
	idx := c.count
	c.count++
	atomic.AddUint32(&c.failed, 1)
	c.msg(idx, id, time.Now(), c.aurora.BgBrightRed("INCOMPLETE").White(), "failed to start:", message)
}

// Manage should be called on the standard output of all test plans. It will
// send the events to standard out and record whether or not the test passed.
func (c *ConsoleOutput) Manage(id string, stdout, stderr io.ReadCloser) {
	idx := c.count
	c.count++

	var (
		ERROR      = c.aurora.BgRed("ERROR").White()
		OK         = c.aurora.BgGreen("OK").White()
		FAIL       = c.aurora.BgRed("FAIL").White()
		INCOMPLETE = c.aurora.BgBrightRed("INCOMPLETE").White()
		MESSAGE    = c.aurora.BgWhite("MESSAGE").Black()
		METRIC     = c.aurora.BgBlue("METRIC").White()
	)

	c.wg.Add(2)
	go func() {
		defer stderr.Close()
		defer c.wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			c.msg(idx, id, time.Now(), ERROR, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			c.msg(idx, id, time.Now(), ERROR, "stderr error: "+err.Error())
		}
	}()

	go func() {
		defer stdout.Close()
		defer c.wg.Done()

		printMsg := func(timestamp int64, kind interface{}, message ...interface{}) {
			c.msg(idx, id, time.Unix(0, timestamp), kind, message...)
		}

		decoder := json.NewDecoder(stdout)
		var event runtime.Event
		var (
			// track both in case a test-case is so broken that it
			// reports both a success and a failure.
			failed = false
			ok     = false
		)
		for {
			if err := decoder.Decode(&event); err != nil {
				now := time.Now().UnixNano()
				if err != io.EOF {
					printMsg(now, ERROR, "stdout error: "+err.Error())
					failed = true
				}
				if !ok && !failed {
					// incomplete.
					printMsg(event.Timestamp, INCOMPLETE)
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
					printMsg(event.Timestamp, OK, event.Result.Reason)
				default:
					failed = true
					printMsg(event.Timestamp, FAIL, event.Result.Outcome, " ", event.Result.Reason)
				}
			} else if event.Metric != nil {
				marshaled, err := json.Marshal(event.Metric)
				if err != nil {
					printMsg(event.Timestamp, ERROR, "malformed metric:", err)
				} else {
					printMsg(event.Timestamp, METRIC, string(marshaled))
				}
			} else {
				printMsg(event.Timestamp, MESSAGE, event.Message)
			}
		}
	}()
}
