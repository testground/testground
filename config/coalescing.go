package config

import (
	"bytes"
	"fmt"
	"reflect"

	"github.com/BurntSushi/toml"
)

type CoalescedConfig []map[string]interface{}

func (c CoalescedConfig) Append(in map[string]interface{}) CoalescedConfig {
	return append(c, in)
}

func (c CoalescedConfig) CoalesceIntoType(typ reflect.Type) (interface{}, error) {
	all := make(map[string]interface{})

	// Copy all values into coalesced map.
	for _, cfg := range c {
		if c == nil {
			continue
		}
		for k, v := range cfg {
			all[k] = v
		}
	}

	// Serialize map into TOML, and then deserialize into the appropriate type.
	buf := new(bytes.Buffer)
	if err := toml.NewEncoder(buf).Encode(all); err != nil {
		return nil, fmt.Errorf("error while encoding into TOML: %w", err)
	}

	v := reflect.New(typ).Interface()
	_, err := toml.DecodeReader(buf, v)
	return v, err
}
