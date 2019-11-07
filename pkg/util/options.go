package util

import (
	"fmt"
	"strconv"
	"strings"
)

// ToOptionsMap converts a slice of ["KEY1=VAL1", "KEY2=VAL2", ...] dictionary
// values into a map of interface{}, where the actual type is determined by
// guessing if guessTypes is true, or else it remains a string.
//
// TODO may need to be extended to support variadic values, returning a
// map[string][]string (a map of string keys to string slices).
func ToOptionsMap(input []string, guessTypes bool) (res map[string]interface{}, err error) {
	res = make(map[string]interface{}, len(input))
	var v interface{}
	for _, d := range input {
		splt := strings.Split(d, "=")
		if len(splt) != 2 {
			return nil, fmt.Errorf("invalid key-value: %s", d)
		}

		v = splt[1]
		if guessTypes {
			// 1. Try to parse as an integer.
			// 2. Try to parse as a float.
			// 3. Try to parse as a bool.
			// 4. It is a string, try to unquote.
			// 5. Default to string value.
			if v, err = strconv.Atoi(splt[1]); err == nil {
			} else if v, err = strconv.ParseFloat(splt[1], 64); err == nil {
			} else if v, err = strconv.ParseBool(splt[1]); err == nil {
			} else if v, err = strconv.Unquote(splt[1]); err == nil { //nolint
			} else {
				v = splt[1]
			}
		}
		res[splt[0]] = v
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
