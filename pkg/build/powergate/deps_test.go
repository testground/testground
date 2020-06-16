package powergate

import (
	"fmt"
	"reflect"
	"testing"
)

var testParseDependencies = []struct {
	input  string
	output map[string]string
}{
	{
		input: `example.com/module/a v1.2`,
		output: map[string]string{
			"example.com/module/a": "v1.2",
		},
	},
	{
		input: `example.com/module/a v1.2

example.com/module/b v2.2`,
		output: map[string]string{
			"example.com/module/a": "v1.2",
			"example.com/module/b": "v2.2",
		},
	},
	{
		input: `example.com/module/a v0.0.0-hashsomething
example.com/module/b v1.2.4
example.com/module/c v2.0.0
example.com/module/d v2.0.0 => ./sdk/runtime`,
		output: map[string]string{
			"example.com/module/a": "v0.0.0-hashsomething",
			"example.com/module/b": "v1.2.4",
			"example.com/module/c": "v2.0.0",
			"example.com/module/d": "v2.0.0 => ./sdk/runtime",
		},
	},
}

func TestParseDependencies(t *testing.T) {
	for i, test := range testParseDependencies {
		val := parseDependencies(test.input)

		if !reflect.DeepEqual(val, test.output) {
			fmt.Print(val)
			t.Errorf("Incorrect value on parseDependencies for test %d", i)
		}
	}
}
