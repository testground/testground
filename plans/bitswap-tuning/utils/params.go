package utils

import (
	"fmt"
	"strconv"
	"strings"
)

func ParseIntArray(value string) ([]int, error) {
	var ints []int
	strs := strings.Split(value, ",")
	for _, str := range strs {
		num, err := strconv.ParseInt(str, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("Could not convert '%s' to integer(s)", strs)
		}
		ints = append(ints, int(num))
	}
	return ints, nil
}
