package sync

import (
	"fmt"
	"reflect"

	capi "github.com/hashicorp/consul/api"
	"github.com/ipfs/testground/logging"
)

// A Subtree represents a subtree of the sync tree for a test run. It is bound
// to a particular subpath pattern, where payloads of a specific type are posted
// and manipulated.
type Subtree struct {
	// PayloadType is the type of the payload. Must be a pointer type.
	PayloadType reflect.Type

	// PathFunc extracts the path to put this value to.
	PathFunc func(val interface{}) string

	// MatchFunc determines whether an incoming kv pair matches this subtree.
	MatchFunc func(*capi.KVPair) bool
}

// AssertType panics if the value doesn't match the expected type.
func (s *Subtree) AssertType(typ reflect.Type) {
	if typ == s.PayloadType {
		return
	}

	err := fmt.Errorf("expected type %s did not match actual type %s", s.PayloadType, typ)
	logging.S().DPanic(err)
	panic(err)
}
