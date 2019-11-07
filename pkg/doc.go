// Package pkg is only meant to be imported by the testground daemon, sidecar
// and the CLI.
//
// Test plans should not depend on package pkg. Everything that a test plan
// needs should be under package sdk.
package pkg
