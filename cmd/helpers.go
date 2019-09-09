package cmd

import (
	"fmt"
	"strings"
)

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
