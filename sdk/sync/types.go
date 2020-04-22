package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/testground/testground/sdk/runtime"
)

// State represents a state in a distributed state machine, identified by a
// unique string within the test case.
type State string

// Key gets the Redis key for this State, contextualized to a set of RunParams.
func (s State) Key(rp *runtime.RunParams) string {
	p := fmt.Sprintf("run:%s:plan:%s:case:%s:states:%s", rp.TestRun, rp.TestPlan, rp.TestCase, string(s))
	return p
}

// Barrier represents a barrier over a State. A Barrier is a synchronisation
// checkpoint that will fire once the `target` number of entries on that state
// have been registered.
type Barrier struct {
	C chan error

	ctx    context.Context
	state  State
	key    string
	target int64
}

// Topic represents a meeting place for test instances to exchange arbitrary
// data.
type Topic struct {
	name string
	typ  reflect.Type
}

// NewTopic constructs a Topic with the provided name, and the type of the
// supplied value, derived via reflect.TypeOf, unless the supplied value is
// already a reflect.Type. This method does not retain actual value from which
// the type is derived.
func NewTopic(name string, typ interface{}) *Topic {
	t, ok := typ.(reflect.Type)
	if !ok {
		t = reflect.TypeOf(typ)
	}
	return &Topic{name, t}
}

// Key gets the key for this Topic, contextualized to a set of RunParams.
func (t Topic) Key(rp *runtime.RunParams) string {
	p := fmt.Sprintf("run:%s:plan:%s:case:%s:topics:%s", rp.TestRun, rp.TestPlan, rp.TestCase, t.name)
	return p
}

func (t Topic) validatePayload(val interface{}) bool {
	ttyp, vtyp := t.typ, reflect.TypeOf(val)
	if ttyp.Kind() == reflect.Ptr {
		ttyp = ttyp.Elem()
	}
	if vtyp.Kind() == reflect.Ptr {
		vtyp = vtyp.Elem()
	}
	return ttyp == vtyp
}

// decodePayload extracts a value of the specified type from incoming json.
func (t Topic) decodePayload(val interface{}) (reflect.Value, error) {
	// Deserialize the value.
	typ := t.typ
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	payload := reflect.New(typ)
	raw, ok := val.(string)
	if !ok {
		panic("payload not a string")
	}
	if err := json.Unmarshal([]byte(raw), payload.Interface()); err != nil {
		return reflect.Value{}, fmt.Errorf("failed to decode as type %s: %s", t.typ, string(raw))
	}
	return payload, nil
}

// Subscription represents a receive channel for data being published in a
// Topic.
type Subscription struct {
	ctx    context.Context
	outCh  reflect.Value
	doneCh chan error

	// sendFn performs a select over outCh and the context, and returns true if
	// we sent the value, or false if the context fired.
	sendFn func(v reflect.Value) (sent bool)

	topic  *Topic
	key    string
	lastid string
}

func (s *Subscription) Done() <-chan error {
	return s.doneCh
}
