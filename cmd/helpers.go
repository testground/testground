package cmd

import (
	"fmt"
	"strings"
)

// toKeyValues converts a slice of ["KEY1=VAL1", "KEY2=VAL2", ...] dictionary
// values into a map of strings.
//
// TODO may need to be extended to support variadic values, returning a
// map[string][]string (a map of string keys to string slices).
func toKeyValues(input []string) (map[string]string, error) {
	res := make(map[string]string, len(input))
	for _, d := range input {
		splt := strings.Split(d, "=")
		if len(splt) != 2 {
			return nil, fmt.Errorf("invalid key-value: %s", d)
		}
		res[splt[0]] = splt[1]
	}
	return res, nil
}
