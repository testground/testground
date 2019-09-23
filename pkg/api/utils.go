package api

import (
	"reflect"
)

// EnumerateOverridableFields returns the fields from a struct type that can be
// overridden, i.e. they bear the `overridable="yes"` tag, and they have an
// explicitly defined toml key.
//
// TODO: can move this to an `enumerable` type that we embed in the
// configuration type itself, e.g.:
//
//   v := GoBuildStrategy{}
//   v.EnumerateOverridableFields()
func EnumerateOverridableFields(typ reflect.Type) (out []string) {
	if typ.Kind() != reflect.Struct {
		return nil
	}
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		// Is this field overridable?
		if v, ok := f.Tag.Lookup("overridable"); ok && v == "yes" {
			// Get the toml key and append it.
			if t, ok := f.Tag.Lookup("toml"); ok {
				out = append(out, t)
			}
		}
	}
	return out
}
