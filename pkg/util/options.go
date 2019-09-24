package util

import (
	"fmt"
	"strings"
)

// ToOptionsMap converts a slice of ["KEY1=VAL1", "KEY2=VAL2", ...] dictionary
// values into a map of strings.
//
// TODO may need to be extended to support variadic values, returning a
// map[string][]string (a map of string keys to string slices).
func ToOptionsMap(input []string) (map[string]interface{}, error) {
	res := make(map[string]interface{}, len(input))
	for _, d := range input {
		splt := strings.Split(d, "=")
		if len(splt) != 2 {
			return nil, fmt.Errorf("invalid key-value: %s", d)
		}
		res[splt[0]] = splt[1]
	}
	return res, nil
}

// ToStringStringMap takes a map[string]interface{} and down-casts it to a
// map[string]string asserting that all values are strings.
func ToStringStringMap(input map[string]interface{}) (map[string]string, error) {
	out := make(map[string]string, len(input))
	for k, v := range input {
		if value, ok := v.(string); ok {
			out[k] = value
		} else {
			return nil, fmt.Errorf("some values are not strings")
		}
	}
	return out, nil
}

// ToOptionsSlice takes an options map and returns a slice of form ["KEY1=VAL1",
// "KEY2=VAL2", ...], suitable for Docker commands.
func ToOptionsSlice(input map[string]string) []string {
	out := make([]string, 0, len(input))
	for k, v := range input {
		out = append(out, k+"="+v)
	}
	return out
}
