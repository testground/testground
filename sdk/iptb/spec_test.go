package iptb

import (
	"reflect"
	"testing"
)

func TestNewTestEnsembleSpec(t *testing.T) {
	spec := NewTestEnsembleSpec()
	if !reflect.DeepEqual(spec, &TestEnsembleSpec{
		tags: make(map[string]NodeOpts),
	}) {
		t.Fail()
	}
}

func TestEnsembleSpecConfig(t *testing.T) {
	spec := NewTestEnsembleSpec()
	spec.AddNodesDefaultConfig(NodeOpts{}, "t1")

	if !reflect.DeepEqual(spec, &TestEnsembleSpec{
		tags: map[string]NodeOpts{
			"t1": NodeOpts{},
		},
	}) {
		t.Fail()
	}
}

func TestEnsembleSpecPanicSameTag(t *testing.T) {
	spec := NewTestEnsembleSpec()

	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("function must have panicked")
		}
	}()

	spec.AddNodesDefaultConfig(NodeOpts{}, "t1")
	spec.AddNodesDefaultConfig(NodeOpts{}, "t1")
}
