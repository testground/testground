package runner

import (
	"strings"
)

// ExposedPorts is a simple type that holds port mappings.
type ExposedPorts map[string]string

// ToEnvVars returns a map that represents these port mappings as environment
// variables, in the form ${LABEL}_PORT=${PORT_NUMBER}.
//
// The result can be piped through conv.ToOptionsSlice to turn it into a slice.
func (e ExposedPorts) ToEnvVars() map[string]string {
	ret := make(map[string]string, len(e))
	for label, port := range e {
		k := strings.ToUpper(strings.TrimSpace(label)) + "_PORT"
		ret[k] = port
	}
	return ret
}
