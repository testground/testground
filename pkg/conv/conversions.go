package conv

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/go-units"
	v1 "k8s.io/api/core/v1"
)

// InferTypedMap takes a map[string]string and attempts to infer the value
// types, converting the input to a map[string]interface{}.
func InferTypedMap(in map[string]string) map[string]interface{} {
	var (
		res = make(map[string]interface{}, len(in))
		t   interface{}
		err error
	)
	for k, v := range in {
		// 1. Try to parse as an integer.
		// 2. Try to parse as a float.
		// 3. Try to parse as a bool.
		// 4. It is a string, try to unquote.
		// 5. Default to string value.
		if t, err = strconv.Atoi(v); err == nil {
		} else if t, err = strconv.ParseFloat(v, 64); err == nil {
		} else if t, err = strconv.ParseBool(v); err == nil {
		} else if t, err = strconv.Unquote(v); err == nil { //nolint
		} else {
			t = v
		}
		res[k] = t
	}
	return res
}

// ParseKeyValues converts a slice of ["KEY1=VAL1", "KEY2=VAL2", ...] dictionary
// values into a map[string]string, reporting errors if the input is malformed.
func ParseKeyValues(in []string) (res map[string]string, err error) {
	res = make(map[string]string, len(in))
	for _, d := range in {
		splt := strings.Split(d, "=")
		if len(splt) < 2 {
			return nil, fmt.Errorf("invalid key-value: %s", d)
		}
		res[splt[0]] = strings.Join(splt[1:], "=")
	}
	return res, nil
}

// CastAsStringMap takes a map[string]interface{} and down-casts it to a
// map[string]string asserting that all values are strings.
func CastAsStringMap(input map[string]interface{}) (map[string]string, error) {
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

func ToEnvVar(input map[string]string) []v1.EnvVar {
	out := make([]v1.EnvVar, len(input))
	ind := 0
	for k, v := range input {
		out[ind].Name = k
		out[ind].Value = v
		ind++
	}
	return out
}

// ToUlimits converts a slice of strings following the Docker ulimit format, to
// the appropriate type. If parsing fails, this function shortcircuits and
// returns an error.
//
// See
// https://docs.docker.com/engine/reference/commandline/run/#set-ulimits-in-container---ulimit
// for more info on format.
func ToUlimits(input []string) ([]*units.Ulimit, error) {
	out := make([]*units.Ulimit, 0, len(input))
	for _, s := range input {
		parsed, err := units.ParseUlimit(s)
		if err != nil {
			return nil, err
		}
		out = append(out, parsed)
	}
	return out, nil
}
