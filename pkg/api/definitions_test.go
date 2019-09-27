package api

import (
	"testing"

	"github.com/davecgh/go-spew/spew"

	"github.com/BurntSushi/toml"
)

func TestManifest(t *testing.T) {
	var plan TestPlanDefinition

	m, err := toml.DecodeFile("../../manifests/dht.toml", &plan)
	if err != nil {
		t.Fatal(err)
	}

	spew.Dump(plan)

	var dockergo GoBuildStrategy
	m.PrimitiveDecode(plan.BuildStrategies["docker:go"].Primitive, &dockergo)

	spew.Dump(dockergo)

}
