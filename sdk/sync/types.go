package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/ipfs/testground/sdk/runtime"
)

type State string

// Key gets the Redis key, contextualized to a set of RunParams.
func (s State) Key(rp *runtime.RunParams) string {
	p := fmt.Sprintf("run:%s:plan:%s:case:%s:states:%s", rp.TestRun, rp.TestPlan, rp.TestCase, string(s))
	return p
}

type Barrier struct {
	C chan error

	ctx    context.Context
	state  State
	key    string
	target int64
}

type Topic struct {
	Name string
	Type reflect.Type
}

// Key gets the key, contextualized to the parent.
func (t Topic) Key(rp *runtime.RunParams) string {
	p := fmt.Sprintf("run:%s:plan:%s:case:%s:topics:%s", rp.TestRun, rp.TestPlan, rp.TestCase, t.Name)
	return p
}

func (t Topic) validatePayload(val interface{}) bool {
	ttyp, vtyp := t.Type, reflect.TypeOf(val)
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
	typ := t.Type
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	payload := reflect.New(typ)
	raw, ok := val.(string)
	if !ok {
		panic("payload not a string")
	}
	if err := json.Unmarshal([]byte(raw), payload.Interface()); err != nil {
		return reflect.Value{}, fmt.Errorf("failed to decode as type %s: %s", t.Type, string(raw))
	}
	return payload, nil
}

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
