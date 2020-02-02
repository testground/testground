package runtime

// Invoke runs the passed test-case and reports the result.
func Invoke(tc func(*RunEnv) error) {
	runenv := CurrentRunEnv()

	// Prepare the event.
	defer func() {
		if err := recover(); err != nil {
			// Handle panics.
			runenv.RecordCrash(err)
		}
	}()

	err := tc(runenv)
	switch err {
	case nil:
		runenv.RecordSuccess()
	default:
		runenv.RecordFailure(err)
	}
}
