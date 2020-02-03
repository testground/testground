package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"
	"time"
)

// Invoke runs the passed test-case and reports the result.
func Invoke(tc func(*RunEnv) error) {
	runenv := CurrentRunEnv()

	// Prepare the event.
	evt := Event{RunEnv: runenv}
	defer func() {
		evt.Timestamp = time.Now().UnixNano()
		if err := recover(); err != nil {
			// Handle panics.
			evt.Result = &Result{OutcomeCrashed, fmt.Sprintf("%s", err), string(debug.Stack())}
		}
		json.NewEncoder(os.Stdout).Encode(evt)
	}()

	err := tc(runenv)
	switch err {
	case nil:
		evt.Result = &Result{OutcomeOK, "", ""}
	default:
		evt.Result = &Result{OutcomeAborted, err.Error(), ""}
	}
}
