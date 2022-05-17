//go:build go1.16 && !v14
// +build go1.16,!v14

package shim

func Version() string {
	return "v16"
}
