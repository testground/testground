package sync

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
)

//
// Each state in this state machine has two edges which represent a failure and success paths.
// To use the state machine, attach a funciton to the node using the OnEnter() method. This method
// will be invoked at the right time when the state is Enter()'ed.
// Each State can represent a phase of the test, with the approprate synchronization abstracted
// away from the test logic.
//
// Example:
// func DoSomething() error {
//  ...
// }
//
// func DoSomethingElse() error {
//  ...
// }
///
// s1 := NewBarrierAllStateMachineNode("main", "state1", runenv)
// s2 := NewBarrierAllStateMachineNode("main", "state2", runenv)
// s1.OnEnter(DoSomething)
// s1.AttachSuccess(s2)
// s2.OnEnter(DoSomethingElse)

var BarrierExecuteError = errors.New("timeout waiting for barrier")

type StateMachineExecutable func() error

type StateMachineNode interface {
	Succeed() StateMachineNode
	Fail(e error) StateMachineNode
	OnEnter(e StateMachineExecutable)
	Enter() error
	AttachSuccess(a StateMachineNode)
	AttachFailure(a StateMachineNode)
}

// TerminalSuccessNode
// This is a state machine endpoint.
// Attaching nodes to this state will do nothing.
// The Enter() method returns nil
type TerminalSuccessNode struct {
}

func (sm TerminalSuccessNode) Succeed() StateMachineNode {
	return sm
}

func (sm TerminalSuccessNode) Fail(e error) StateMachineNode {
	return sm
}

func (sm TerminalSuccessNode) OnEnter(e StateMachineExecutable) {
}

func (sm TerminalSuccessNode) Enter() error {
	return nil
}

func (sm TerminalSuccessNode) AttachSuccess(a StateMachineNode) {
}

func (sm TerminalSuccessNode) AttachFailure(a StateMachineNode) {
}

// TerminalFailureNode
// This is a state machine endpoint.
// Attaching nodes to this state will do nothing.
// The Enter() method returns an error message.
type TerminalFailureNode struct {
}

func (sm TerminalFailureNode) Succeed() StateMachineNode {
	return sm
}

func (sm TerminalFailureNode) Fail(e error) StateMachineNode {
	return sm
}

func (sm TerminalFailureNode) OnEnter(e StateMachineExecutable) {
}

func (sm TerminalFailureNode) Enter() error {
	// This isn't a very descriptive message.
	// Rather than providing more context here, maybe other error messages are more descriptive?
	return errors.New("State machine reached a terminal state.")
}

func (sm TerminalFailureNode) AttachSuccess(a StateMachineNode) {
}

func (sm TerminalFailureNode) AttachFailure(a StateMachineNode) {
}

// A BarrierStateMachineNode is a fully synchronous state machine.
// This state machine waits until all nodes are in the same state.
// Most likely, the operand to the Enter method will be a closure method encapsulating
// the task to be performed at a certain stage of the test plan.
type BarrierAllStateMachineNode struct {
	Name        string
	stname      string
	successNode StateMachineNode
	failureNode StateMachineNode
	enterfunc   StateMachineExecutable
	runenv      *runtime.RunEnv
}

func (sm BarrierAllStateMachineNode) Succeed() StateMachineNode {
	return sm.successNode
}

func (sm BarrierAllStateMachineNode) Fail(err error) StateMachineNode {
	return sm.failureNode
}

func (sm BarrierAllStateMachineNode) AttachSuccess(a StateMachineNode) {
	sm.successNode = a
}

func (sm BarrierAllStateMachineNode) AttachFailure(a StateMachineNode) {
	sm.failureNode = a
}

func (sm BarrierAllStateMachineNode) OnEnter(e StateMachineExecutable) {
	sm.enterfunc = e
}

func (sm BarrierAllStateMachineNode) Enter() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(2)*time.Minute)
	defer cancel()

	watcher, writer := MustWatcherWriter(ctx, sm.runenv)

	s := State(sm.Name)
	writer.SignalEntry(ctx, s)

	// TODO, we should watch for -failed states, and fail when those are encountered
	// For now, the context timeout will work.
	err := <-watcher.Barrier(ctx, s, int64(sm.runenv.TestInstanceCount))
	if err != nil {
		failmsg := fmt.Sprintf("%s-failed", sm.Name)
		fstate := State(failmsg)
		writer.SignalEntry(ctx, fstate)
		sm.runenv.RecordFailure(errors.New(failmsg))
		return sm.Fail(err).Enter()
	}
	err = sm.enterfunc()
	if err != nil {
		return sm.Fail(err).Enter()
	}
	return sm.Succeed().Enter()
}

func NewBarrierAllStateMachineNode(stname string, statename string, runenv *runtime.RunEnv) *BarrierAllStateMachineNode {
	sm := BarrierAllStateMachineNode{
		Name:        statename,
		stname:      stname,
		successNode: TerminalSuccessNode{},
		failureNode: TerminalFailureNode{},
		enterfunc:   func() error { return nil },
		runenv:      runenv,
	}
	return &sm
}
