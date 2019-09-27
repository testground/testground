package sync

import (
	"fmt"
	"reflect"
)

// A Subtree represents a subtree of the sync tree for a test run. It is bound
// to a particular subpath pattern, where payloads of a specific type are posted
// and manipulated.
type Subtree struct {
	// GroupKey is the name of the group the keys in this subtree belong to.
	GroupKey string

	// PayloadType is the type of the payload. Must be a pointer type.
	PayloadType reflect.Type

	// KeyFunc returns the key of this entry within the subtree, optionally
	// deriving it from the supplied payload.
	KeyFunc func(payload interface{}) string
}

// AssertType errors if the value doesn't match the expected type.
func (s *Subtree) AssertType(typ reflect.Type) error {
	if typ == s.PayloadType {
		return nil
	}

	return fmt.Errorf("expected type %s did not match actual type %s", s.PayloadType, typ)
}

func (s *Subtree) String() string {
	return fmt.Sprintf("Subtree{GroupKey: %s, PayloadType: %s}", s.GroupKey, s.PayloadType)
}
