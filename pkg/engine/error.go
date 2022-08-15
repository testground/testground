package engine

import "fmt"

type TaskExecutionError struct {
	TaskType   string
	WrappedErr error
}

func (e *TaskExecutionError) Error() string {
	return fmt.Sprintf("task of type %s cancelled: %v", e.TaskType, e.WrappedErr.Error())
}
