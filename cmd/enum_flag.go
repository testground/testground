package cmd

import (
	"fmt"
	"strings"

	"github.com/urfave/cli"
)

// EnumValue is a cli.Generic flag type that enforces flag values from an enum.
type EnumValue struct {
	Allowed []string
	Default string
	v       string
}

var _ cli.Generic = (*EnumValue)(nil)

func (e *EnumValue) Set(value string) error {
	for _, a := range e.Allowed {
		if a == value {
			e.v = value
			return nil
		}
	}
	return fmt.Errorf("allowed values are %s; got: %s", strings.Join(e.Allowed, ", "), value)
}

func (e *EnumValue) String() string {
	if e.v == "" {
		return e.Default
	}
	return e.v
}
